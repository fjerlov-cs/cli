package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"os/signal"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/humio/cli/internal/api"
	"github.com/humio/cli/prompt"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

func newSearchCmd() *cobra.Command {
	var (
		start        string
		end          string
		live         bool
		fmtStr       string
		noWrap       bool
		noProgress   bool
		jsonProgress bool
	)

	cmd := &cobra.Command{
		Use:   "search [flags] <repo> <query>",
		Short: "Search",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			repository := args[0]
			queryString := args[1]
			client := NewApiClient(cmd)

			ctx := contextCancelledOnInterrupt(context.Background())

			// get the search start time, used for json output
			startMillis := time.Now().UnixMilli()

			// set noProgress if getting json
			if jsonProgress {
				noProgress = true
			}

			// run in lambda func to be able to defer and delete the query job
			err := func() error {
				id, err := client.QueryJobs().Create(repository, api.Query{
					QueryString:                queryString,
					Start:                      start,
					End:                        end,
					Live:                       live,
					ShowQueryEventDistribution: true,
				})

				if err != nil {
					return err
				}

				var progress *queryResultProgressBar
				if !noProgress {
					progress = newQueryResultProgressBar()
				}

				defer func(id string) {
					// Humio will eventually delete the query when we stop polling and we can't do much about errors here.
					_ = client.QueryJobs().Delete(repository, id)
				}(id)

				var result api.QueryResult
				poller := queryJobPoller{
					queryJobs:  client.QueryJobs(),
					repository: repository,
					id:         id,
				}
				result, err = poller.WaitAndPollContext(ctx)

				if err != nil {
					return err
				}

				var printer interface {
					print(api.QueryResult)
				}

				if result.Metadata.IsAggregate {
					printer = newAggregatePrinter(cmd.OutOrStdout(), noWrap)
				} else {
					printer = newEventListPrinter(cmd.OutOrStdout(), fmtStr)
				}

				for !result.Done {
					if progress != nil {
						progress.Update(result)
					}
					if jsonProgress {
						jsonProgress, _ := printQueryResultProgressJson(result, args, startMillis)
						fmt.Printf("%s\n", jsonProgress)
					}
					result, err = poller.WaitAndPollContext(ctx)
					if err != nil {
						return err
					}
				}

				if progress != nil {
					progress.Update(result)
					progress.Finish()
				}

				if jsonProgress {
					jsonProgress, _ := printQueryResultProgressJson(result, args, startMillis)
					fmt.Printf("%s\n", jsonProgress)
				}

				// no output if using jsonProgress
				if !jsonProgress {
					printer.print(result)
				}

				if live {
					for {
						result, err = poller.WaitAndPollContext(ctx)
						if err != nil {
							return err
						}

						printer.print(result)
					}
				}

				return nil
			}()

			if err == context.Canceled {
				err = nil
			}

			if queryError, ok := err.(api.QueryError); ok {
				cmd.PrintErrf("There was an error in your query string:\n\n%s\n", queryError.Error())
				os.Exit(1)
			}

			exitOnError(cmd, err, "error running search")
		},
	}

	cmd.Flags().StringVarP(&start, "start", "s", "10m", "Query start time")
	cmd.Flags().StringVarP(&end, "end", "e", "", "Query end time")
	cmd.Flags().BoolVarP(&live, "live", "l", false, "Run a live search and keep outputting until interrupted.")
	cmd.Flags().StringVarP(&fmtStr, "fmt", "f", "{@timestamp} {@rawstring}", "Format string if the result is an event list\n"+
		"Insert fields by wrapping field names in brackets, e.g. {@timestamp}\n"+
		"Limited format modifiers are supported such as {@timestamp:40} which will right align and left pad @timestamp to 40 characters.\n"+
		"{@timestamp:-40} left aligns and right pads to 40 characters.")
	cmd.Flags().BoolVarP(&noWrap, "no-wrap", "n", false, "Do not autowrap long strings.")
	cmd.Flags().BoolVar(&noProgress, "no-progress", false, "Do not should progress information.")
	cmd.Flags().BoolVar(&jsonProgress, "json-progress", false, "Print progress in json format. This disables progress and output, useful for logging search metadata.")

	return cmd
}

func contextCancelledOnInterrupt(ctx context.Context) context.Context {
	ctx, cancel := context.WithCancel(ctx)

	sigC := make(chan os.Signal, 1)
	signal.Notify(sigC, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigC
		cancel()
	}()

	return ctx
}

type queryResultProgressBar struct {
	bar      *prompt.ProgressBar
	epsValue float64
	bpsValue float64
	hits     uint64
}

func newQueryResultProgressBar() *queryResultProgressBar {
	b := &queryResultProgressBar{}
	b.bar = prompt.NewProgressBar(
		prompt.ProgressOptionDescription("Searching..."),
		prompt.ProgressOptionAppendAdditionalInfo(b.additionalInfoBps),
		prompt.ProgressOptionAppendAdditionalInfo(b.additionalInfoEps),
		prompt.ProgressOptionAppendAdditionalInfo(b.additionalInfoHits),
	)
	b.epsValue = math.NaN()
	b.bpsValue = math.NaN()
	b.bar.Start()
	return b
}

func (b *queryResultProgressBar) Update(result api.QueryResult) {
	if result.Metadata.TimeMillis > 0 {
		b.epsValue = float64(result.Metadata.ProcessedEvents) / float64(result.Metadata.TimeMillis) * 1000
		b.bpsValue = float64(result.Metadata.ProcessedBytes) / float64(result.Metadata.TimeMillis) * 1000
	}

	b.hits = result.Metadata.EventCount

	b.bar.Set(result.Metadata.WorkDone, result.Metadata.TotalWork)
}

func (b *queryResultProgressBar) additionalInfoEps() string {
	if !math.IsNaN(b.epsValue) {
		v, suffix := prompt.AddSISuffix(b.epsValue, false)
		return fmt.Sprintf("%.1f %s events/s", v, suffix)
	}
	return ""
}

func (b *queryResultProgressBar) additionalInfoBps() string {
	if !math.IsNaN(b.bpsValue) {
		v, suffix := prompt.AddSISuffix(b.bpsValue, true)
		return fmt.Sprintf("%.1f %sB/s", v, suffix)
	}
	return ""
}

func (b *queryResultProgressBar) additionalInfoHits() string {
	v, suffix := prompt.AddSISuffix(float64(b.hits), false)
	return fmt.Sprintf("%.1f %s events", v, suffix)
}

func (b *queryResultProgressBar) Finish() {
	b.bar.Finish()
}

type queryResultProgressJson struct {
	Timestamp   int64   `json:"timestamp"`
	StartMillis int64   `json:"startMillis"`
	Repo        string  `json:"repo"`
	QueryString string  `json:"queryString"`
	Start       uint64  `json:"start"`
	End         uint64  `json:"end"`
	TotalWork   uint64  `json:"totalWork"`
	WorkDone    uint64  `json:"workDone"`
	TimeMillis  uint64  `json:"timeMillis"`
	EpsValue    float64 `json:"epsValue"`
	BpsValue    float64 `json:"bpsValue"`
	EventCount  uint64  `json:"eventCount"`
	Done        bool    `json:"done"`
}

func printQueryResultProgressJson(result api.QueryResult, args []string, startMillis int64) (string, error) {
	var epsValue, bpsValue float64

	if result.Metadata.TimeMillis > 0 {
		epsValue = float64(result.Metadata.ProcessedEvents) / float64(result.Metadata.TimeMillis) * 1000
		bpsValue = float64(result.Metadata.ProcessedBytes) / float64(result.Metadata.TimeMillis) * 1000
	}

	timestamp := time.Now().UnixMilli()

	jsonResult := &queryResultProgressJson{
		Timestamp:   timestamp,
		StartMillis: startMillis,
		Repo:        args[0],
		QueryString: args[1],
		Start:       result.Metadata.QueryStart,
		End:         result.Metadata.QueryEnd,
		TotalWork:   result.Metadata.TotalWork,
		WorkDone:    result.Metadata.WorkDone,
		TimeMillis:  result.Metadata.TimeMillis,
		EpsValue:    epsValue,
		BpsValue:    bpsValue,
		EventCount:  result.Metadata.EventCount,
		Done:        result.Done,
	}

	data, err := json.Marshal(jsonResult)
	return string(data), err
}

type queryJobPoller struct {
	queryJobs  *api.QueryJobs
	repository string
	id         string
	nextPoll   time.Time
}

func (q *queryJobPoller) WaitAndPollContext(ctx context.Context) (api.QueryResult, error) {
	select {
	case <-time.After(time.Until(q.nextPoll)):
	case <-ctx.Done():
		return api.QueryResult{}, ctx.Err()
	}

	result, err := q.queryJobs.PollContext(ctx, q.repository, q.id)
	if err != nil {
		return result, err
	}

	q.nextPoll = time.Now().Add(time.Duration(result.Metadata.PollAfter) * time.Millisecond)

	return result, err
}

var fieldPrinters = map[string]func(v interface{}) (string, bool){
	"@timestamp": func(v interface{}) (string, bool) {
		fv, ok := v.(float64)
		if !ok {
			return "", false
		}

		sec, msec := int64(fv)/1000, int64(fv)%1000

		t := time.Unix(sec, msec*1000000)

		return t.Format(time.RFC3339Nano), true
	},
}

type eventListPrinter struct {
	printedIds     map[string]bool
	printFields    []string
	w              io.Writer
	printEventFunc func(io.Writer, map[string]interface{})
	fmt            string
}

func newEventListPrinter(w io.Writer, fmt string) *eventListPrinter {
	e := &eventListPrinter{
		printedIds: map[string]bool{},
		w:          w,
	}

	re := regexp.MustCompile(`(\{[^\}]+\})`)
	e.fmt = re.ReplaceAllStringFunc(fmt, func(f string) string {
		field := f[1 : len(f)-1]
		arg := ""

		if strings.Contains(field, ":") {
			pieces := strings.SplitN(field, ":", 2)
			field, arg = pieces[0], pieces[1]
		}

		e.printFields = append(e.printFields, field)
		return "%" + arg + "s"
	})

	e.initPrintFunc()
	return e
}

func (p *eventListPrinter) initPrintFunc() {
	var printers []func(map[string]interface{}) string
	for _, f := range p.printFields {
		f := f
		if printer, hasPrinter := fieldPrinters[f]; hasPrinter {
			printers = append(printers, func(m map[string]interface{}) string {
				v := m[f]
				if str, ok := printer(v); ok {
					return str
				} else {
					return fmt.Sprint(v)
				}
			})
		} else {
			printers = append(printers, func(m map[string]interface{}) string {
				v := m[f]
				return fmt.Sprint(v)
			})
		}
	}

	p.printEventFunc = func(w io.Writer, m map[string]interface{}) {
		fmtArgs := make([]interface{}, len(printers))
		for i, printer := range printers {
			fmtArgs[i] = printer(m)
		}
		fmt.Fprintf(w, p.fmt+"\n", fmtArgs...)
	}
}

func (p *eventListPrinter) print(result api.QueryResult) {
	sort.Slice(result.Events, func(i, j int) bool {
		tsI, hasTsI := result.Events[i]["@timestamp"].(float64)
		tsJ, hasTsJ := result.Events[j]["@timestamp"].(float64)

		switch {
		case hasTsI && hasTsJ:
			return tsI < tsJ
		case !hasTsJ:
			return false
		case !hasTsI:
			return true
		default:
			return false
		}
	})

	for _, e := range result.Events {
		id, hasID := e["@id"].(string)
		if hasID && !p.printedIds[id] {
			p.printEventFunc(p.w, e)
			p.printedIds[id] = true
		} else if !hasID {
			p.printEventFunc(p.w, e)
		}
	}
}

type aggregatePrinter struct {
	w       io.Writer
	columns []string
	noWrap  bool
}

func newAggregatePrinter(w io.Writer, noWrap bool) *aggregatePrinter {
	return &aggregatePrinter{
		w:      w,
		noWrap: noWrap,
	}
}

func (p *aggregatePrinter) print(result api.QueryResult) {
	if len(result.Metadata.FieldOrder) > 0 {
		p.columns = result.Metadata.FieldOrder
	} else {
		f := p.columns
		m := map[string]bool{}
		for _, e := range result.Events {
			for k := range e {
				if !m[k] {
					f = append(f, k)
					m[k] = true
				}
			}
		}
		p.columns = f
	}

	if len(p.columns) == 0 {
		return
	}

	if len(p.columns) == 1 && len(result.Events) == 1 {
		// single column, single result, just print it
		fmt.Fprintln(p.w, result.Events[0][p.columns[0]])
		return
	}

	t := tablewriter.NewWriter(p.w)
	t.SetAutoFormatHeaders(false)
	t.SetBorder(false)
	t.SetHeader(p.columns)
	t.SetHeaderLine(false)
	t.SetAutoWrapText(!p.noWrap)

	for _, e := range result.Events {
		var r []string
		for _, i := range p.columns {
			v, hasField := e[i]
			if hasField {
				r = append(r, fmt.Sprint(v))
			} else {
				r = append(r, "")
			}
		}
		t.Append(r)
	}

	t.Render()
	fmt.Fprintln(p.w)
}
