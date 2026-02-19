package api

import (
	"context"
	"fmt"
	"reflect"

	"github.com/humio/cli/internal/api/humiographql"
)

const LogScaleVersionWithS3Action = "1.221.0"

type ActionType string

const (
	ActionTypeEmail            ActionType = "EmailAction"
	ActionTypeHumioRepo        ActionType = "HumioRepoAction"
	ActionTypeOpsGenie         ActionType = "OpsGenieAction"
	ActionTypePagerDuty        ActionType = "PagerDutyAction"
	ActionTypeSlack            ActionType = "SlackAction"
	ActionTypeSlackPostMessage ActionType = "SlackPostMessageAction"
	ActionTypeVictorOps        ActionType = "VictorOpsAction"
	ActionTypeUploadFile       ActionType = "UploadFileAction"
	ActionTypeWebhook          ActionType = "WebhookAction"
	ActionTypeS3               ActionType = "S3Action"
)

type Actions struct {
	client *Client
}

type EmailAction struct {
	Recipients      []string
	SubjectTemplate *string
	BodyTemplate    *string
	UseProxy        bool
	Labels          []string
}

type HumioRepoAction struct {
	IngestToken string
	Labels      []string
}

type OpsGenieAction struct {
	ApiUrl   string
	GenieKey string
	UseProxy bool
	Labels   []string
}

type PagerDutyAction struct {
	Severity   string
	RoutingKey string
	UseProxy   bool
	Labels     []string
}

type SlackField struct {
	FieldName string
	Value     string
}

type SlackAction struct {
	Url      string
	Fields   []SlackField
	UseProxy bool
	Labels   []string
}

type SlackPostMessageAction struct {
	ApiToken string
	Channels []string
	Fields   []SlackField
	UseProxy bool
	Labels   []string
}

type UploadFileAction struct {
	FileName string
	Labels   []string
}

type VictorOpsAction struct {
	MessageType string
	NotifyUrl   string
	UseProxy    bool
	Labels      []string
}

type HttpHeader struct {
	Header string
	Value  string
}

type WebhookAction struct {
	Method       string
	Url          string
	Headers      []HttpHeader
	BodyTemplate string
	IgnoreSSL    bool
	UseProxy     bool
	Labels       []string
}

type S3Action struct {
	RoleArn        string
	AwsRegion      string
	BucketName     string
	FileName       string
	OutputFormat   string
	OutputMetadata bool
	UseProxy       bool
	Labels         []string
}

type Action struct {
	Type ActionType
	ID   string `yaml:"-"`
	Name string

	EmailAction            EmailAction            `yaml:"emailAction,omitempty"`
	HumioRepoAction        HumioRepoAction        `yaml:"humioRepoAction,omitempty"`
	OpsGenieAction         OpsGenieAction         `yaml:"opsGenieAction,omitempty"`
	PagerDutyAction        PagerDutyAction        `yaml:"pagerDutyAction,omitempty"`
	SlackAction            SlackAction            `yaml:"slackAction,omitempty"`
	SlackPostMessageAction SlackPostMessageAction `yaml:"slackPostMessageAction,omitempty"`
	VictorOpsAction        VictorOpsAction        `yaml:"victorOpsAction,omitempty"`
	UploadFileAction       UploadFileAction       `yaml:"uploadFileAction,omitempty"`
	WebhookAction          WebhookAction          `yaml:"webhookAction,omitempty"`
	S3Action               S3Action               `yaml:"s3Action,omitempty"`
}

// GetLabels returns the labels from the specific action type
func (a Action) GetLabels() []string {
	switch a.Type {
	case ActionTypeEmail:
		return a.EmailAction.Labels
	case ActionTypeHumioRepo:
		return a.HumioRepoAction.Labels
	case ActionTypeOpsGenie:
		return a.OpsGenieAction.Labels
	case ActionTypePagerDuty:
		return a.PagerDutyAction.Labels
	case ActionTypeSlack:
		return a.SlackAction.Labels
	case ActionTypeSlackPostMessage:
		return a.SlackPostMessageAction.Labels
	case ActionTypeVictorOps:
		return a.VictorOpsAction.Labels
	case ActionTypeUploadFile:
		return a.UploadFileAction.Labels
	case ActionTypeWebhook:
		return a.WebhookAction.Labels
	case ActionTypeS3:
		return a.S3Action.Labels
	default:
		return nil
	}
}

func (c *Client) Actions() *Actions { return &Actions{client: c} }

func (n *Actions) serverSupportsS3Actions() (bool, error) {
	status, err := n.client.Status()
	if err != nil {
		return false, err
	}
	return status.AtLeast(LogScaleVersionWithS3Action)
}

func (n *Actions) List(searchDomainName string) ([]Action, error) {
	s3Supported, err := n.serverSupportsS3Actions()
	if err != nil {
		return nil, fmt.Errorf("unable to determine server version: %w", err)
	}
	if s3Supported {
		return n.listWithS3(searchDomainName)
	}
	return n.listWithoutS3(searchDomainName)
}

func (n *Actions) listWithS3(searchDomainName string) ([]Action, error) {
	resp, err := humiographql.ListActions(context.Background(), n.client, searchDomainName)
	if err != nil {
		return nil, err
	}
	respSearchDomain := resp.GetSearchDomain()
	respSearchDomainActions := respSearchDomain.GetActions()
	actions := make([]Action, len(respSearchDomainActions))
	for idx, action := range respSearchDomainActions {
		switch v := action.(type) {
		case *humiographql.ListActionsSearchDomainActionsEmailAction:
			actions[idx] = Action{
				Type: ActionType(*v.GetTypename()),
				ID:   v.GetId(),
				Name: v.GetName(),
				EmailAction: EmailAction{
					Recipients:      v.GetRecipients(),
					SubjectTemplate: v.GetSubjectTemplate(),
					BodyTemplate:    v.GetEmailBodyTemplate(),
					UseProxy:        v.GetUseProxy(),
					Labels:          v.GetLabels(),
				},
			}
		case *humiographql.ListActionsSearchDomainActionsHumioRepoAction:
			actions[idx] = Action{
				Type: ActionType(*v.GetTypename()),
				ID:   v.GetId(),
				Name: v.GetName(),
				HumioRepoAction: HumioRepoAction{
					IngestToken: v.GetIngestToken(),
					Labels:      v.GetLabels(),
				},
			}
		case *humiographql.ListActionsSearchDomainActionsOpsGenieAction:
			actions[idx] = Action{
				Type: ActionType(*v.GetTypename()),
				ID:   v.GetId(),
				Name: v.GetName(),
				OpsGenieAction: OpsGenieAction{
					ApiUrl:   v.GetApiUrl(),
					GenieKey: v.GetGenieKey(),
					UseProxy: v.GetUseProxy(),
					Labels:   v.GetLabels(),
				},
			}
		case *humiographql.ListActionsSearchDomainActionsPagerDutyAction:
			actions[idx] = Action{
				Type: ActionType(*v.GetTypename()),
				ID:   v.GetId(),
				Name: v.GetName(),
				PagerDutyAction: PagerDutyAction{
					Severity:   v.GetSeverity(),
					RoutingKey: v.GetRoutingKey(),
					UseProxy:   v.GetUseProxy(),
					Labels:     v.GetLabels(),
				},
			}
		case *humiographql.ListActionsSearchDomainActionsSlackAction:
			fields := make([]SlackField, len(v.GetFields()))
			for jdx, field := range v.GetFields() {
				fields[jdx] = SlackField{
					FieldName: field.GetFieldName(),
					Value:     field.GetValue(),
				}
			}
			actions[idx] = Action{
				Type: ActionType(*v.GetTypename()),
				ID:   v.GetId(),
				Name: v.GetName(),
				SlackAction: SlackAction{
					Url:      v.GetUrl(),
					Fields:   fields,
					UseProxy: v.GetUseProxy(),
					Labels:   v.GetLabels(),
				},
			}
		case *humiographql.ListActionsSearchDomainActionsSlackPostMessageAction:
			fields := make([]SlackField, len(v.GetFields()))
			for jdx, field := range v.GetFields() {
				fields[jdx] = SlackField{
					FieldName: field.GetFieldName(),
					Value:     field.GetValue(),
				}
			}
			actions[idx] = Action{
				Type: ActionType(*v.GetTypename()),
				ID:   v.GetId(),
				Name: v.GetName(),
				SlackPostMessageAction: SlackPostMessageAction{
					ApiToken: v.GetApiToken(),
					Channels: v.GetChannels(),
					Fields:   fields,
					UseProxy: v.GetUseProxy(),
					Labels:   v.GetLabels(),
				},
			}
		case *humiographql.ListActionsSearchDomainActionsVictorOpsAction:
			actions[idx] = Action{
				Type: ActionType(*v.GetTypename()),
				ID:   v.GetId(),
				Name: v.GetName(),
				VictorOpsAction: VictorOpsAction{
					MessageType: v.GetMessageType(),
					NotifyUrl:   v.GetNotifyUrl(),
					UseProxy:    v.GetUseProxy(),
					Labels:      v.GetLabels(),
				},
			}
		case *humiographql.ListActionsSearchDomainActionsUploadFileAction:
			actions[idx] = Action{
				Type: ActionType(*v.GetTypename()),
				ID:   v.GetId(),
				Name: v.GetName(),
				UploadFileAction: UploadFileAction{
					FileName: v.GetFileName(),
					Labels:   v.GetLabels(),
				},
			}
		case *humiographql.ListActionsSearchDomainActionsWebhookAction:
			headers := make([]HttpHeader, len(v.GetHeaders()))
			for jdx, header := range v.GetHeaders() {
				headers[jdx] = HttpHeader{
					Header: header.GetHeader(),
					Value:  header.GetValue(),
				}
			}
			actions[idx] = Action{
				Type: ActionType(*v.GetTypename()),
				ID:   v.GetId(),
				Name: v.GetName(),
				WebhookAction: WebhookAction{
					Method:       v.GetMethod(),
					Url:          v.GetUrl(),
					Headers:      headers,
					BodyTemplate: v.GetWebhookBodyTemplate(),
					IgnoreSSL:    v.GetIgnoreSSL(),
					UseProxy:     v.GetUseProxy(),
					Labels:       v.GetLabels(),
				},
			}
		case *humiographql.ListActionsSearchDomainActionsS3Action:
			actions[idx] = Action{
				Type: ActionType(*v.GetTypename()),
				ID:   v.GetId(),
				Name: v.GetName(),
				S3Action: S3Action{
					RoleArn:        v.GetRoleArn(),
					AwsRegion:      v.GetAwsRegion(),
					BucketName:     v.GetBucketName(),
					FileName:       v.GetFileName(),
					OutputFormat:   string(v.GetOutputFormat()),
					OutputMetadata: v.GetOutputMetadata(),
					UseProxy:       v.GetUseProxy(),
					Labels:         v.GetLabels(),
				},
			}
		default:
			actions[idx] = Action{
				Type: ActionType(*v.GetTypename()),
				ID:   v.GetId(),
				Name: v.GetName(),
			}
		}
	}

	return actions, nil
}

func (n *Actions) listWithoutS3(searchDomainName string) ([]Action, error) {
	resp, err := humiographql.ListActionsWithoutS3(context.Background(), n.client, searchDomainName)
	if err != nil {
		return nil, err
	}
	respSearchDomain := resp.GetSearchDomain()
	respSearchDomainActions := respSearchDomain.GetActions()
	actions := make([]Action, len(respSearchDomainActions))
	for idx, action := range respSearchDomainActions {
		switch v := action.(type) {
		case *humiographql.ListActionsWithoutS3SearchDomainActionsEmailAction:
			actions[idx] = Action{
				Type: ActionType(*v.GetTypename()),
				ID:   v.GetId(),
				Name: v.GetName(),
				EmailAction: EmailAction{
					Recipients:      v.GetRecipients(),
					SubjectTemplate: v.GetSubjectTemplate(),
					BodyTemplate:    v.GetEmailBodyTemplate(),
					UseProxy:        v.GetUseProxy(),
					Labels:          v.GetLabels(),
				},
			}
		case *humiographql.ListActionsWithoutS3SearchDomainActionsHumioRepoAction:
			actions[idx] = Action{
				Type: ActionType(*v.GetTypename()),
				ID:   v.GetId(),
				Name: v.GetName(),
				HumioRepoAction: HumioRepoAction{
					IngestToken: v.GetIngestToken(),
					Labels:      v.GetLabels(),
				},
			}
		case *humiographql.ListActionsWithoutS3SearchDomainActionsOpsGenieAction:
			actions[idx] = Action{
				Type: ActionType(*v.GetTypename()),
				ID:   v.GetId(),
				Name: v.GetName(),
				OpsGenieAction: OpsGenieAction{
					ApiUrl:   v.GetApiUrl(),
					GenieKey: v.GetGenieKey(),
					UseProxy: v.GetUseProxy(),
					Labels:   v.GetLabels(),
				},
			}
		case *humiographql.ListActionsWithoutS3SearchDomainActionsPagerDutyAction:
			actions[idx] = Action{
				Type: ActionType(*v.GetTypename()),
				ID:   v.GetId(),
				Name: v.GetName(),
				PagerDutyAction: PagerDutyAction{
					Severity:   v.GetSeverity(),
					RoutingKey: v.GetRoutingKey(),
					UseProxy:   v.GetUseProxy(),
					Labels:     v.GetLabels(),
				},
			}
		case *humiographql.ListActionsWithoutS3SearchDomainActionsSlackAction:
			fields := make([]SlackField, len(v.GetFields()))
			for jdx, field := range v.GetFields() {
				fields[jdx] = SlackField{
					FieldName: field.GetFieldName(),
					Value:     field.GetValue(),
				}
			}
			actions[idx] = Action{
				Type: ActionType(*v.GetTypename()),
				ID:   v.GetId(),
				Name: v.GetName(),
				SlackAction: SlackAction{
					Url:      v.GetUrl(),
					Fields:   fields,
					UseProxy: v.GetUseProxy(),
					Labels:   v.GetLabels(),
				},
			}
		case *humiographql.ListActionsWithoutS3SearchDomainActionsSlackPostMessageAction:
			fields := make([]SlackField, len(v.GetFields()))
			for jdx, field := range v.GetFields() {
				fields[jdx] = SlackField{
					FieldName: field.GetFieldName(),
					Value:     field.GetValue(),
				}
			}
			actions[idx] = Action{
				Type: ActionType(*v.GetTypename()),
				ID:   v.GetId(),
				Name: v.GetName(),
				SlackPostMessageAction: SlackPostMessageAction{
					ApiToken: v.GetApiToken(),
					Channels: v.GetChannels(),
					Fields:   fields,
					UseProxy: v.GetUseProxy(),
					Labels:   v.GetLabels(),
				},
			}
		case *humiographql.ListActionsWithoutS3SearchDomainActionsVictorOpsAction:
			actions[idx] = Action{
				Type: ActionType(*v.GetTypename()),
				ID:   v.GetId(),
				Name: v.GetName(),
				VictorOpsAction: VictorOpsAction{
					MessageType: v.GetMessageType(),
					NotifyUrl:   v.GetNotifyUrl(),
					UseProxy:    v.GetUseProxy(),
					Labels:      v.GetLabels(),
				},
			}
		case *humiographql.ListActionsWithoutS3SearchDomainActionsUploadFileAction:
			actions[idx] = Action{
				Type: ActionType(*v.GetTypename()),
				ID:   v.GetId(),
				Name: v.GetName(),
				UploadFileAction: UploadFileAction{
					FileName: v.GetFileName(),
					Labels:   v.GetLabels(),
				},
			}
		case *humiographql.ListActionsWithoutS3SearchDomainActionsWebhookAction:
			headers := make([]HttpHeader, len(v.GetHeaders()))
			for jdx, header := range v.GetHeaders() {
				headers[jdx] = HttpHeader{
					Header: header.GetHeader(),
					Value:  header.GetValue(),
				}
			}
			actions[idx] = Action{
				Type: ActionType(*v.GetTypename()),
				ID:   v.GetId(),
				Name: v.GetName(),
				WebhookAction: WebhookAction{
					Method:       v.GetMethod(),
					Url:          v.GetUrl(),
					Headers:      headers,
					BodyTemplate: v.GetWebhookBodyTemplate(),
					IgnoreSSL:    v.GetIgnoreSSL(),
					UseProxy:     v.GetUseProxy(),
					Labels:       v.GetLabels(),
				},
			}
		default:
			actions[idx] = Action{
				Type: ActionType(*v.GetTypename()),
				ID:   v.GetId(),
				Name: v.GetName(),
			}
		}
	}

	return actions, nil
}

func (n *Actions) Add(searchDomainName string, newAction *Action) (*Action, error) {
	if newAction == nil {
		return nil, fmt.Errorf("action must not be nil")
	}

	if !reflect.ValueOf(newAction.EmailAction).IsZero() {
		resp, err := humiographql.CreateEmailAction(
			context.Background(),
			n.client,
			searchDomainName,
			newAction.Name,
			newAction.EmailAction.Recipients,
			newAction.EmailAction.SubjectTemplate,
			newAction.EmailAction.BodyTemplate,
			newAction.EmailAction.UseProxy,
			newAction.EmailAction.Labels,
		)
		if err != nil {
			return nil, err
		}

		respUpdate := resp.GetCreateEmailAction()
		return &Action{
			Type: ActionTypeEmail,
			ID:   respUpdate.GetId(),
			Name: respUpdate.GetName(),
			EmailAction: EmailAction{
				Recipients:      respUpdate.GetRecipients(),
				SubjectTemplate: respUpdate.GetSubjectTemplate(),
				BodyTemplate:    respUpdate.GetBodyTemplate(),
				UseProxy:        respUpdate.GetUseProxy(),
				Labels:          respUpdate.GetLabels(),
			},
		}, nil
	}

	if !reflect.ValueOf(newAction.HumioRepoAction).IsZero() {
		resp, err := humiographql.CreateHumioRepoAction(
			context.Background(),
			n.client,
			searchDomainName,
			newAction.Name,
			newAction.HumioRepoAction.IngestToken,
			newAction.HumioRepoAction.Labels,
		)
		if err != nil {
			return nil, err
		}

		respUpdate := resp.GetCreateHumioRepoAction()
		return &Action{
			Type: ActionTypeHumioRepo,
			ID:   respUpdate.GetId(),
			Name: respUpdate.GetName(),
			HumioRepoAction: HumioRepoAction{
				IngestToken: respUpdate.GetIngestToken(),
				Labels:      respUpdate.GetLabels(),
			},
		}, nil
	}

	if !reflect.ValueOf(newAction.OpsGenieAction).IsZero() {
		resp, err := humiographql.CreateOpsGenieAction(
			context.Background(),
			n.client,
			searchDomainName,
			newAction.Name,
			newAction.OpsGenieAction.ApiUrl,
			newAction.OpsGenieAction.GenieKey,
			newAction.OpsGenieAction.UseProxy,
			newAction.OpsGenieAction.Labels,
		)
		if err != nil {
			return nil, err
		}

		respUpdate := resp.GetCreateOpsGenieAction()
		return &Action{
			Type: ActionTypeOpsGenie,
			ID:   respUpdate.GetId(),
			Name: respUpdate.GetName(),
			OpsGenieAction: OpsGenieAction{
				ApiUrl:   respUpdate.GetApiUrl(),
				GenieKey: respUpdate.GetGenieKey(),
				UseProxy: respUpdate.GetUseProxy(),
				Labels:   respUpdate.GetLabels(),
			},
		}, nil
	}

	if !reflect.ValueOf(newAction.PagerDutyAction).IsZero() {
		resp, err := humiographql.CreatePagerDutyAction(
			context.Background(),
			n.client,
			searchDomainName,
			newAction.Name,
			newAction.PagerDutyAction.Severity,
			newAction.PagerDutyAction.RoutingKey,
			newAction.PagerDutyAction.UseProxy,
			newAction.PagerDutyAction.Labels,
		)
		if err != nil {
			return nil, err
		}

		respUpdate := resp.GetCreatePagerDutyAction()
		return &Action{
			Type: ActionTypePagerDuty,
			ID:   respUpdate.GetId(),
			Name: respUpdate.GetName(),
			PagerDutyAction: PagerDutyAction{
				Severity:   respUpdate.GetSeverity(),
				RoutingKey: respUpdate.GetRoutingKey(),
				UseProxy:   respUpdate.GetUseProxy(),
				Labels:     respUpdate.GetLabels(),
			},
		}, nil
	}

	if !reflect.ValueOf(newAction.SlackAction).IsZero() {
		fields := make([]humiographql.SlackFieldEntryInput, len(newAction.SlackAction.Fields))
		for idx, field := range newAction.SlackAction.Fields {
			fields[idx] = humiographql.SlackFieldEntryInput{
				FieldName: field.FieldName,
				Value:     field.Value,
			}
		}
		resp, err := humiographql.CreateSlackAction(
			context.Background(),
			n.client,
			searchDomainName,
			newAction.Name,
			fields,
			newAction.SlackAction.Url,
			newAction.SlackAction.UseProxy,
			newAction.SlackAction.Labels,
		)
		if err != nil {
			return nil, err
		}

		respUpdate := resp.GetCreateSlackAction()
		respUpdateFields := respUpdate.GetFields()
		fieldsUpdate := make([]SlackField, len(respUpdateFields))
		for idx, field := range respUpdateFields {
			fieldsUpdate[idx] = SlackField{
				FieldName: field.GetFieldName(),
				Value:     field.GetValue(),
			}
		}
		return &Action{
			Type: ActionTypeSlack,
			ID:   respUpdate.GetId(),
			Name: respUpdate.GetName(),
			SlackAction: SlackAction{
				Fields:   fieldsUpdate,
				Url:      respUpdate.GetUrl(),
				UseProxy: respUpdate.GetUseProxy(),
				Labels:   respUpdate.GetLabels(),
			},
		}, nil
	}

	if !reflect.ValueOf(newAction.SlackPostMessageAction).IsZero() {
		fields := make([]humiographql.SlackFieldEntryInput, len(newAction.SlackPostMessageAction.Fields))
		for idx, field := range newAction.SlackPostMessageAction.Fields {
			fields[idx] = humiographql.SlackFieldEntryInput{
				FieldName: field.FieldName,
				Value:     field.Value,
			}
		}
		resp, err := humiographql.CreateSlackPostMessageAction(
			context.Background(),
			n.client,
			searchDomainName,
			newAction.Name,
			newAction.SlackPostMessageAction.ApiToken,
			newAction.SlackPostMessageAction.Channels,
			fields,
			newAction.SlackPostMessageAction.UseProxy,
			newAction.SlackPostMessageAction.Labels,
		)
		if err != nil {
			return nil, err
		}

		respUpdate := resp.GetCreateSlackPostMessageAction()
		respUpdateFields := respUpdate.GetFields()
		fieldsUpdate := make([]SlackField, len(respUpdateFields))
		for idx, field := range respUpdateFields {
			fieldsUpdate[idx] = SlackField{
				FieldName: field.GetFieldName(),
				Value:     field.GetValue(),
			}
		}
		return &Action{
			Type: ActionTypeSlackPostMessage,
			ID:   respUpdate.GetId(),
			Name: respUpdate.GetName(),
			SlackPostMessageAction: SlackPostMessageAction{
				ApiToken: respUpdate.GetApiToken(),
				Channels: respUpdate.GetChannels(),
				Fields:   fieldsUpdate,
				UseProxy: respUpdate.GetUseProxy(),
				Labels:   respUpdate.GetLabels(),
			},
		}, nil
	}

	if !reflect.ValueOf(newAction.VictorOpsAction).IsZero() {
		resp, err := humiographql.CreateVictorOpsAction(
			context.Background(),
			n.client,
			searchDomainName,
			newAction.Name,
			newAction.VictorOpsAction.MessageType,
			newAction.VictorOpsAction.NotifyUrl,
			newAction.VictorOpsAction.UseProxy,
			newAction.VictorOpsAction.Labels,
		)
		if err != nil {
			return nil, err
		}

		respUpdate := resp.GetCreateVictorOpsAction()
		return &Action{
			Type: ActionTypeVictorOps,
			ID:   respUpdate.GetId(),
			Name: respUpdate.GetName(),
			VictorOpsAction: VictorOpsAction{
				MessageType: respUpdate.GetMessageType(),
				NotifyUrl:   respUpdate.GetNotifyUrl(),
				UseProxy:    respUpdate.GetUseProxy(),
				Labels:      respUpdate.GetLabels(),
			},
		}, nil
	}

	if !reflect.ValueOf(newAction.UploadFileAction).IsZero() {
		resp, err := humiographql.CreateUploadFileAction(
			context.Background(),
			n.client,
			searchDomainName,
			newAction.Name,
			newAction.UploadFileAction.FileName,
			newAction.UploadFileAction.Labels,
		)
		if err != nil {
			return nil, err
		}

		respUpdate := resp.GetCreateUploadFileAction()
		return &Action{
			Type: ActionTypeUploadFile,
			ID:   respUpdate.GetId(),
			Name: respUpdate.GetName(),
			UploadFileAction: UploadFileAction{
				FileName: respUpdate.GetFileName(),
				Labels:   respUpdate.GetLabels(),
			},
		}, nil
	}

	if !reflect.ValueOf(newAction.WebhookAction).IsZero() {
		headers := make([]humiographql.HttpHeaderEntryInput, len(newAction.WebhookAction.Headers))
		for idx, header := range newAction.WebhookAction.Headers {
			headers[idx] = humiographql.HttpHeaderEntryInput{
				Header: header.Header,
				Value:  header.Value,
			}
		}
		resp, err := humiographql.CreateWebhookAction(
			context.Background(),
			n.client,
			searchDomainName,
			newAction.Name,
			newAction.WebhookAction.Url,
			newAction.WebhookAction.Method,
			headers,
			newAction.WebhookAction.BodyTemplate,
			newAction.WebhookAction.IgnoreSSL,
			newAction.WebhookAction.UseProxy,
			newAction.WebhookAction.Labels,
		)
		if err != nil {
			return nil, err
		}

		respUpdate := resp.GetCreateWebhookAction()
		respUpdateHeaders := respUpdate.GetHeaders()
		fieldsUpdate := make([]HttpHeader, len(respUpdateHeaders))
		for idx, header := range respUpdateHeaders {
			fieldsUpdate[idx] = HttpHeader{
				Header: header.GetHeader(),
				Value:  header.GetValue(),
			}
		}
		return &Action{
			Type: ActionTypeWebhook,
			ID:   respUpdate.GetId(),
			Name: respUpdate.GetName(),
			WebhookAction: WebhookAction{
				Url:          respUpdate.GetUrl(),
				Method:       respUpdate.GetMethod(),
				Headers:      fieldsUpdate,
				BodyTemplate: respUpdate.GetBodyTemplate(),
				IgnoreSSL:    respUpdate.GetIgnoreSSL(),
				UseProxy:     respUpdate.GetUseProxy(),
				Labels:       respUpdate.GetLabels(),
			},
		}, nil
	}

	if !reflect.ValueOf(newAction.S3Action).IsZero() {
		s3Supported, err := n.serverSupportsS3Actions()
		if err != nil {
			return nil, fmt.Errorf("unable to determine server version: %w", err)
		}
		if !s3Supported {
			return nil, fmt.Errorf("S3 actions require LogScale version %s or later", LogScaleVersionWithS3Action)
		}
		resp, err := humiographql.CreateS3Action(
			context.Background(),
			n.client,
			searchDomainName,
			newAction.Name,
			newAction.S3Action.RoleArn,
			newAction.S3Action.AwsRegion,
			newAction.S3Action.BucketName,
			newAction.S3Action.FileName,
			humiographql.S3ActionEventOutputFormat(newAction.S3Action.OutputFormat),
			newAction.S3Action.OutputMetadata,
			newAction.S3Action.UseProxy,
			newAction.S3Action.Labels,
		)
		if err != nil {
			return nil, err
		}

		respUpdate := resp.GetCreateS3Action()
		return &Action{
			Type: ActionTypeS3,
			ID:   respUpdate.GetId(),
			Name: respUpdate.GetName(),
			S3Action: S3Action{
				RoleArn:        respUpdate.GetRoleArn(),
				AwsRegion:      respUpdate.GetAwsRegion(),
				BucketName:     respUpdate.GetBucketName(),
				FileName:       respUpdate.GetFileName(),
				OutputFormat:   string(respUpdate.GetOutputFormat()),
				OutputMetadata: respUpdate.GetOutputMetadata(),
				UseProxy:       respUpdate.GetUseProxy(),
				Labels:         respUpdate.GetLabels(),
			},
		}, nil
	}

	return nil, fmt.Errorf("no action details specified or unsupported action type used")
}

func (n *Actions) Get(searchDomainName, actionName string) (*Action, error) {
	actions, err := n.List(searchDomainName)
	if err != nil {
		return nil, fmt.Errorf("unable to list actions: %w", err)
	}
	for _, action := range actions {
		if action.Name == actionName {
			return &action, nil
		}
	}

	return nil, ActionNotFound(actionName)
}

func (n *Actions) Delete(searchDomainName, actionName string) error {
	actions, err := n.List(searchDomainName)
	if err != nil {
		return fmt.Errorf("unable to list actions: %w", err)
	}
	var actionID string
	for _, action := range actions {
		if action.Name == actionName {
			actionID = action.ID
			break
		}
	}
	if actionID == "" {
		return ActionNotFound(actionID)
	}

	_, err = humiographql.DeleteActionByID(context.Background(), n.client, searchDomainName, actionID)
	if err != nil {
		return err
	}

	return nil
}
