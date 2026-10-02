package provider

import (
	"context"
	"os"
	"path/filepath"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/resoftware/terraform-provider-staticform/internal/client"
)

// toRequest builds a client.FormRequest from the Terraform plan model.
func (r *formResource) toRequest(ctx context.Context, m formResourceModel) (client.FormRequest, diag.Diagnostics) {
	var diags diag.Diagnostics

	body := client.FormRequest{
		Name:           m.Name.ValueString(),
		SubmissionMode: strPtr(optStr(m.SubmissionMode)),
	}

	if !m.CaptchaType.IsNull() {
		body.CaptchaType = strPtr(m.CaptchaType.ValueString())
	}
	if !m.CaptchaSecretKey.IsNull() {
		body.CaptchaSecretKey = strPtr(m.CaptchaSecretKey.ValueString())
	}
	if !m.EnableHoneypot.IsNull() {
		v := m.EnableHoneypot.ValueBool()
		body.EnableHoneypot = &v
	}
	if !m.HoneypotFieldName.IsNull() {
		body.HoneypotFieldName = strPtr(m.HoneypotFieldName.ValueString())
	}
	if !m.EnableLanguageDetection.IsNull() {
		v := m.EnableLanguageDetection.ValueBool()
		body.EnableLanguageDetection = &v
	}
	langs, d := stringList(ctx, m.ExpectedLanguages)
	diags.Append(d...)
	body.ExpectedLanguages = langs

	ldf, dldf := stringList(ctx, m.LanguageDetectionFields)
	diags.Append(dldf...)
	body.LanguageDetectionFields = ldf

	body.EnableAllowedDomains = boolPtr(m.EnableAllowedDomains)
	domains, dd := stringList(ctx, m.AllowedDomains)
	diags.Append(dd...)
	body.AllowedDomains = explicitList(domains)

	body.EnableAiSpamReview = boolPtr(m.EnableAiSpamReview)
	excluded, dex := stringList(ctx, m.AiSpamReviewExcluded)
	diags.Append(dex...)
	body.AiSpamReviewExcludedFields = explicitList(excluded)

	if len(m.Payment) > 0 {
		ps, dp := paymentToClient(ctx, m.Payment[0])
		diags.Append(dp...)
		body.PaymentSettings = ps
	}

	for _, f := range m.Fields {
		field := client.Field{
			Name:     f.Name.ValueString(),
			Type:     f.Type.ValueString(),
			Rule:     valueOr(f.Rule, "None"),
			Required: f.Required.ValueBool(),
		}
		vc, d := stringMap(ctx, f.ValidationConfig)
		diags.Append(d...)
		op, d2 := stringMap(ctx, f.Options)
		diags.Append(d2...)
		if vc != nil || op != nil {
			field.Config = &client.FieldConfig{ValidationConfig: vc, Options: op}
		}
		body.Fields = append(body.Fields, field)
	}

	for _, a := range m.SubmitActions {
		action, d := r.actionToClient(ctx, a)
		diags.Append(d...)
		body.SubmitActions = append(body.SubmitActions, action)
	}
	if body.SubmitActions == nil {
		body.SubmitActions = []client.SubmitAction{}
	}

	if len(m.Redirect) > 0 {
		body.RedirectSettings = redirectToClient(m.Redirect[0])
	}

	return body, diags
}

func (r *formResource) actionToClient(ctx context.Context, a submitActionModel) (client.SubmitAction, diag.Diagnostics) {
	var diags diag.Diagnostics
	action := client.SubmitAction{
		Type:    a.Type.ValueString(),
		Name:    a.Name.ValueString(),
		Enabled: valueBoolOr(a.Enabled, true),
	}
	if !a.ID.IsNull() && !a.ID.IsUnknown() && a.ID.ValueString() != "" {
		id := a.ID.ValueString()
		action.ID = &id
	}
	if len(a.RunCondition) > 0 {
		action.RunCondition = runConditionToClient(a.RunCondition[0])
	}

	switch a.Type.ValueString() {
	case client.ActionSendEmail:
		action.Recipient = optStr(a.Recipient)
		action.ReplyTo = strPtr(optStr(a.ReplyTo))
		action.Subject = strPtr(optStr(a.Subject))
		action.BodyTemplate = optStr(a.BodyTemplate)
		action.BodyBuilderState = strPtr(optStr(a.BodyBuilderState))
		action.DisableBranding = a.DisableBranding.ValueBool()
		action.EmailDomainID = strPtr(optStr(a.EmailDomainID))
		action.SMTPConnectionID = strPtr(optStr(a.SMTPConnectionID))
		action.FromEmail = strPtr(optStr(a.FromEmail))
		action.FromName = strPtr(optStr(a.FromName))
		if len(a.EmailTemplate) > 0 {
			action.CustomEmailTemplate = emailTemplateToClient(a.EmailTemplate[0])
		}
		for _, fa := range a.FieldAttachments {
			action.FieldAttachments = append(action.FieldAttachments, client.FieldAttachmentConfig{
				FieldName: fa.FieldName.ValueString(),
				Condition: conditionFromBlocks(fa.Condition),
			})
		}
		for _, sa := range a.StaticAttachments {
			action.StaticAttachments = append(action.StaticAttachments, client.StaticEmailAttachment{
				S3Key:       sa.S3Key.ValueString(),
				FileName:    sa.FileName.ValueString(),
				ContentType: sa.ContentType.ValueString(),
				FileSize:    sa.FileSize.ValueInt64(),
				Condition:   conditionFromBlocks(sa.Condition),
			})
		}
	case client.ActionTriggerWebhook:
		action.WebhookURL = optStr(a.WebhookURL)
		action.HTTPMethod = valueOr(a.HTTPMethod, "Post")
		action.ContentType = valueOr(a.ContentType, "Json")
		action.BodyTemplate = optStr(a.BodyTemplate)
		hdrs, d := stringMap(ctx, a.Headers)
		diags.Append(d...)
		action.Headers = hdrs
	case client.ActionSyncGoogleSheets:
		action.GoogleOAuthConnectionID = strPtr(optStr(a.GoogleConnectionID))
		action.SpreadsheetID = optStr(a.SpreadsheetID)
		action.SheetName = optStr(a.SheetName)
		action.IncludeSubmissionID = valueBoolOr(a.IncludeSubmissionID, true)
		action.IncludeCreatedAt = valueBoolOr(a.IncludeCreatedAt, true)
		action.WriteHeaderIfEmpty = valueBoolOr(a.WriteHeaderIfEmpty, true)
		for _, cm := range a.ColumnMappings {
			action.ColumnMappings = append(action.ColumnMappings, client.GoogleSheetsColumnMapping{
				ColumnHeader:  cm.ColumnHeader.ValueString(),
				FormFieldName: cm.FormFieldName.ValueString(),
			})
		}
	case client.ActionSyncNotion:
		action.NotionConnectionID = strPtr(optStr(a.NotionConnectionID))
		action.DatabaseID = optStr(a.DatabaseID)
		action.DatabaseName = optStr(a.DatabaseName)
		for _, pm := range a.PropertyMappings {
			action.PropertyMappings = append(action.PropertyMappings, client.NotionPropertyMapping{
				PropertyName:  pm.PropertyName.ValueString(),
				PropertyType:  pm.PropertyType.ValueString(),
				FormFieldName: pm.FormFieldName.ValueString(),
			})
		}
	}
	return action, diags
}

// conditionToModel rebuilds the single-group condition model from the API's
// nested condition group (used by run conditions and attachment conditions).
func conditionToModel(g *client.ConditionGroup) []runConditionModel {
	if g == nil || len(g.Groups) == 0 {
		return nil
	}
	sub := g.Groups[0]
	rc := runConditionModel{Connector: types.StringValue(sub.Connector)}
	for _, c := range sub.Conditions {
		rc.Conditions = append(rc.Conditions, conditionModel{
			FieldName:     types.StringValue(c.FieldName),
			Operator:      types.StringValue(c.Operator),
			Value:         nullIfEmpty(c.Value),
			CaseSensitive: types.BoolValue(c.CaseSensitive),
		})
	}
	return []runConditionModel{rc}
}

// conditionFromBlocks builds the API condition group from an optional single
// run-condition block (attachment conditions reuse the run-condition shape).
func conditionFromBlocks(blocks []runConditionModel) *client.ConditionGroup {
	if len(blocks) == 0 {
		return nil
	}
	return runConditionToClient(blocks[0])
}

func runConditionToClient(rc runConditionModel) *client.ConditionGroup {
	connector := valueOr(rc.Connector, "And")
	sub := client.ConditionSubGroup{Connector: connector}
	for _, c := range rc.Conditions {
		sub.Conditions = append(sub.Conditions, client.Condition{
			FieldName:     c.FieldName.ValueString(),
			Operator:      c.Operator.ValueString(),
			Value:         optStr(c.Value),
			CaseSensitive: c.CaseSensitive.ValueBool(),
		})
	}
	return &client.ConditionGroup{Connector: "And", Groups: []client.ConditionSubGroup{sub}}
}

func emailTemplateToClient(m emailTemplateModel) *client.EmailTemplateCustomization {
	return &client.EmailTemplateCustomization{
		PrimaryColor:              strPtr(optStr(m.PrimaryColor)),
		HeaderTextColor:           strPtr(optStr(m.HeaderTextColor)),
		FooterBackgroundColor:     strPtr(optStr(m.FooterBackgroundColor)),
		FooterTextColor:           strPtr(optStr(m.FooterTextColor)),
		BodyBackgroundColor:       strPtr(optStr(m.BodyBackgroundColor)),
		BodyTextColor:             strPtr(optStr(m.BodyTextColor)),
		PrimaryColorDark:          strPtr(optStr(m.PrimaryColorDark)),
		HeaderTextColorDark:       strPtr(optStr(m.HeaderTextColorDark)),
		FooterBackgroundColorDark: strPtr(optStr(m.FooterBackgroundColorDark)),
		FooterTextColorDark:       strPtr(optStr(m.FooterTextColorDark)),
		BodyBackgroundColorDark:   strPtr(optStr(m.BodyBackgroundColorDark)),
		BodyTextColorDark:         strPtr(optStr(m.BodyTextColorDark)),
		LogoURL:                   strPtr(optStr(m.LogoURL)),
		LogoWidth:                 int64Ptr(m.LogoWidth),
		LogoHeight:                int64Ptr(m.LogoHeight),
		HideLogo:                  boolPtr(m.HideLogo),
		HeaderText:                strPtr(optStr(m.HeaderText)),
		HideHeaderText:            boolPtr(m.HideHeaderText),
		SubtitleText:              strPtr(optStr(m.SubtitleText)),
		HideSubtitle:              boolPtr(m.HideSubtitle),
		FooterText:                strPtr(optStr(m.FooterText)),
		SentByText:                strPtr(optStr(m.SentByText)),
		HideSentBy:                boolPtr(m.HideSentBy),
		ShowFormID:                boolPtr(m.ShowFormID),
	}
}

func emailTemplateToModel(c *client.EmailTemplateCustomization) emailTemplateModel {
	return emailTemplateModel{
		PrimaryColor:              strFromPtr(c.PrimaryColor),
		HeaderTextColor:           strFromPtr(c.HeaderTextColor),
		FooterBackgroundColor:     strFromPtr(c.FooterBackgroundColor),
		FooterTextColor:           strFromPtr(c.FooterTextColor),
		BodyBackgroundColor:       strFromPtr(c.BodyBackgroundColor),
		BodyTextColor:             strFromPtr(c.BodyTextColor),
		PrimaryColorDark:          strFromPtr(c.PrimaryColorDark),
		HeaderTextColorDark:       strFromPtr(c.HeaderTextColorDark),
		FooterBackgroundColorDark: strFromPtr(c.FooterBackgroundColorDark),
		FooterTextColorDark:       strFromPtr(c.FooterTextColorDark),
		BodyBackgroundColorDark:   strFromPtr(c.BodyBackgroundColorDark),
		BodyTextColorDark:         strFromPtr(c.BodyTextColorDark),
		LogoURL:                   strFromPtr(c.LogoURL),
		LogoWidth:                 int64FromPtr(c.LogoWidth),
		LogoHeight:                int64FromPtr(c.LogoHeight),
		HideLogo:                  boolFromPtr(c.HideLogo),
		HeaderText:                strFromPtr(c.HeaderText),
		HideHeaderText:            boolFromPtr(c.HideHeaderText),
		SubtitleText:              strFromPtr(c.SubtitleText),
		HideSubtitle:              boolFromPtr(c.HideSubtitle),
		FooterText:                strFromPtr(c.FooterText),
		SentByText:                strFromPtr(c.SentByText),
		HideSentBy:                boolFromPtr(c.HideSentBy),
		ShowFormID:                boolFromPtr(c.ShowFormID),
	}
}

func redirectToClient(m redirectModel) *client.RedirectSettings {
	rs := &client.RedirectSettings{}
	if len(m.Success) > 0 {
		rs.Success = redirectBranchToClient(m.Success[0])
	}
	if len(m.Error) > 0 {
		rs.Error = redirectBranchToClient(m.Error[0])
	}
	return rs
}

func redirectBranchToClient(b redirectBranchModel) *client.RedirectConfig {
	return &client.RedirectConfig{
		Type:      b.Type.ValueString(),
		CustomURL: strPtr(optStr(b.CustomURL)),
		Message:   strPtr(optStr(b.Message)),
	}
}

// apply populates the Terraform model from an API form response.
func (r *formResource) apply(ctx context.Context, f *client.Form, m *formResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(f.ID)
	m.Name = types.StringValue(f.Name)
	m.SubmissionMode = types.StringValue(f.SubmissionMode)
	m.ServerSubmissionSecret = strFromPtr(f.ServerSubmissionSecret)
	m.CaptchaType = types.StringValue(f.CaptchaType)
	// captcha_secret_key is write-only: although the API returns it, we never
	// read it back into state. Reading it would (a) store the live secret in
	// state and (b) make import/refresh plan to overwrite or clear it (config
	// can't echo a sensitive value). Leaving the prior config/state value
	// untouched means an apply only ever sends the secret when it is explicitly
	// set, so existing captcha secrets in production are never changed.
	m.EnableHoneypot = types.BoolValue(f.EnableHoneypot)
	m.HoneypotFieldName = types.StringValue(f.HoneypotFieldName)
	m.EnableLanguageDetection = types.BoolValue(f.EnableLanguageDetection)
	if len(f.ExpectedLanguages) > 0 {
		m.ExpectedLanguages = toStringList(f.ExpectedLanguages)
	} else {
		m.ExpectedLanguages = types.ListNull(types.StringType)
	}
	if len(f.LanguageDetectionFields) > 0 {
		m.LanguageDetectionFields = toStringList(f.LanguageDetectionFields)
	} else {
		m.LanguageDetectionFields = types.ListNull(types.StringType)
	}
	// Older API versions don't return these settings. Keep the planned/prior
	// value then, so a provider release never breaks on an API that lags behind.
	m.EnableAllowedDomains = boolFromResponse(f.EnableAllowedDomains, m.EnableAllowedDomains, types.BoolValue(false))
	m.AllowedDomains = listFromResponse(f.AllowedDomains, m.AllowedDomains)
	m.EnableAiSpamReview = boolFromResponse(f.EnableAiSpamReview, m.EnableAiSpamReview, types.BoolNull())
	m.AiSpamReviewExcluded = listFromResponse(f.AiSpamReviewExcludedFields, m.AiSpamReviewExcluded)
	if len(f.Tags) > 0 {
		m.Tags = toStringSet(f.Tags)
	} else {
		m.Tags = types.SetNull(types.StringType)
	}

	m.Fields = nil
	for _, f := range f.Fields {
		fm := fieldModel{
			Name:             types.StringValue(f.Name),
			Type:             types.StringValue(f.Type),
			Rule:             types.StringValue(f.Rule),
			Required:         types.BoolValue(f.Required),
			ValidationConfig: types.MapNull(types.StringType),
			Options:          types.MapNull(types.StringType),
		}
		if f.Config != nil {
			fm.ValidationConfig = toStringMap(f.Config.ValidationConfig)
			fm.Options = toStringMap(f.Config.Options)
		}
		m.Fields = append(m.Fields, fm)
	}

	// The API does not guarantee submit-action order matches the configured
	// order, so reorder the response to follow the prior model (config/state)
	// order — matching by server ID first, then by name — to avoid spurious
	// positional diffs. Newly added actions fall to the end.
	ordered := orderSubmitActions(m.SubmitActions, f.SubmitActions)
	m.SubmitActions = nil
	for _, a := range ordered {
		m.SubmitActions = append(m.SubmitActions, actionToModel(a))
	}

	if f.RedirectSettings != nil && (f.RedirectSettings.Success != nil || f.RedirectSettings.Error != nil) {
		m.Redirect = []redirectModel{redirectToModel(f.RedirectSettings)}
	} else {
		m.Redirect = nil
	}

	if f.PaymentSettings != nil && f.PaymentSettings.Enabled {
		m.Payment = []paymentModel{paymentToModel(f.PaymentSettings)}
	} else {
		m.Payment = nil
	}

	return diags
}

func paymentToClient(ctx context.Context, p paymentModel) (*client.PaymentSettings, diag.Diagnostics) {
	var diags diag.Diagnostics
	ps := &client.PaymentSettings{
		Enabled:                true,
		ConnectionID:           strPtr(optStr(p.ConnectionID)),
		Currency:               optStr(p.Currency),
		Mode:                   optStr(p.Mode),
		CustomerEmailFieldName: strPtr(optStr(p.CustomerEmailFieldName)),
	}
	for _, r := range p.FixedRule {
		ps.FixedRules = append(ps.FixedRules, client.FixedPaymentRule{
			AmountCents: r.AmountCents.ValueInt64(),
			Description: strPtr(optStr(r.Description)),
		})
	}
	for _, r := range p.FieldAmountRule {
		ps.FieldAmountRules = append(ps.FieldAmountRules, client.FieldAmountPaymentRule{
			AmountFieldName: optStr(r.AmountFieldName),
			Description:     strPtr(optStr(r.Description)),
		})
	}
	for _, li := range p.LineItem {
		priceMap := map[string]int64{}
		d := li.PriceMap.ElementsAs(ctx, &priceMap, false)
		diags.Append(d...)
		ps.LineItems = append(ps.LineItems, client.PaymentLineItem{
			FieldName:         optStr(li.FieldName),
			PriceMap:          priceMap,
			QuantityFieldName: strPtr(optStr(li.QuantityFieldName)),
			Description:       strPtr(optStr(li.Description)),
		})
	}
	return ps, diags
}

func paymentToModel(p *client.PaymentSettings) paymentModel {
	m := paymentModel{
		ConnectionID:           strFromPtr(p.ConnectionID),
		Currency:               types.StringValue(p.Currency),
		Mode:                   types.StringValue(p.Mode),
		CustomerEmailFieldName: strFromPtr(p.CustomerEmailFieldName),
	}
	for _, r := range p.FixedRules {
		m.FixedRule = append(m.FixedRule, fixedRuleModel{
			AmountCents: types.Int64Value(r.AmountCents),
			Description: strFromPtr(r.Description),
		})
	}
	for _, r := range p.FieldAmountRules {
		m.FieldAmountRule = append(m.FieldAmountRule, fieldAmountRuleModel{
			AmountFieldName: types.StringValue(r.AmountFieldName),
			Description:     strFromPtr(r.Description),
		})
	}
	for _, li := range p.LineItems {
		priceMap := map[string]int64{}
		for k, v := range li.PriceMap {
			priceMap[k] = v
		}
		pm, _ := types.MapValueFrom(context.Background(), types.Int64Type, priceMap)
		m.LineItem = append(m.LineItem, lineItemModel{
			FieldName:         types.StringValue(li.FieldName),
			PriceMap:          pm,
			QuantityFieldName: strFromPtr(li.QuantityFieldName),
			Description:       strFromPtr(li.Description),
		})
	}
	return m
}

// orderSubmitActions returns incoming actions ordered to follow prior (the
// configured/state order), matching each prior entry by server ID when present
// and falling back to name. Unmatched incoming actions are appended in their
// original order.
func orderSubmitActions(prior []submitActionModel, incoming []client.SubmitAction) []client.SubmitAction {
	used := make([]bool, len(incoming))
	out := make([]client.SubmitAction, 0, len(incoming))

	for _, p := range prior {
		pid := optStr(p.ID)
		pname := optStr(p.Name)
		idx := -1
		if pid != "" {
			for i, a := range incoming {
				if !used[i] && a.ID != nil && *a.ID == pid {
					idx = i
					break
				}
			}
		}
		if idx == -1 && pname != "" {
			for i, a := range incoming {
				if !used[i] && a.Name == pname {
					idx = i
					break
				}
			}
		}
		if idx >= 0 {
			used[idx] = true
			out = append(out, incoming[idx])
		}
	}
	for i, a := range incoming {
		if !used[i] {
			out = append(out, a)
		}
	}
	return out
}

func actionToModel(a client.SubmitAction) submitActionModel {
	m := submitActionModel{
		ID:                 strFromPtr(a.ID),
		Type:               types.StringValue(a.Type),
		Name:               types.StringValue(a.Name),
		Enabled:            types.BoolValue(a.Enabled),
		Recipient:          nullIfEmpty(a.Recipient),
		ReplyTo:            strFromPtr(a.ReplyTo),
		Subject:            strFromPtr(a.Subject),
		BodyTemplate:       nullIfEmpty(a.BodyTemplate),
		WebhookURL:         nullIfEmpty(a.WebhookURL),
		HTTPMethod:         nullIfEmpty(a.HTTPMethod),
		ContentType:        nullIfEmpty(a.ContentType),
		Headers:            toStringMap(a.Headers),
		GoogleConnectionID: strFromPtr(a.GoogleOAuthConnectionID),
		SpreadsheetID:      nullIfEmpty(a.SpreadsheetID),
		SheetName:          nullIfEmpty(a.SheetName),
		NotionConnectionID: strFromPtr(a.NotionConnectionID),
		DatabaseID:         nullIfEmpty(a.DatabaseID),
		DatabaseName:       nullIfEmpty(a.DatabaseName),
		EmailDomainID:      strFromPtr(a.EmailDomainID),
		SMTPConnectionID:   strFromPtr(a.SMTPConnectionID),
	}

	// Type-specific computed-default fields live on a single shared block, so set
	// them to their schema default for action types that don't use them; otherwise
	// an omitted attribute on (say) a webhook action would diff against the default.
	isEmail := a.Type == client.ActionSendEmail
	isSheets := a.Type == client.ActionSyncGoogleSheets
	if isEmail {
		m.DisableBranding = types.BoolValue(a.DisableBranding)
		m.FromEmail = strFromPtr(a.FromEmail)
		m.FromName = strFromPtr(a.FromName)
		m.BodyBuilderState = strFromPtr(a.BodyBuilderState)
		if a.CustomEmailTemplate != nil {
			m.EmailTemplate = []emailTemplateModel{emailTemplateToModel(a.CustomEmailTemplate)}
		}
	} else {
		m.DisableBranding = types.BoolValue(false)
		m.FromEmail = types.StringNull()
		m.FromName = types.StringNull()
		m.BodyBuilderState = types.StringNull()
	}
	if isSheets {
		m.IncludeSubmissionID = types.BoolValue(a.IncludeSubmissionID)
		m.IncludeCreatedAt = types.BoolValue(a.IncludeCreatedAt)
		m.WriteHeaderIfEmpty = types.BoolValue(a.WriteHeaderIfEmpty)
	} else {
		m.IncludeSubmissionID = types.BoolValue(true)
		m.IncludeCreatedAt = types.BoolValue(true)
		m.WriteHeaderIfEmpty = types.BoolValue(true)
	}

	for _, cm := range a.ColumnMappings {
		m.ColumnMappings = append(m.ColumnMappings, columnMappingModel{
			ColumnHeader:  types.StringValue(cm.ColumnHeader),
			FormFieldName: types.StringValue(cm.FormFieldName),
		})
	}
	for _, pm := range a.PropertyMappings {
		m.PropertyMappings = append(m.PropertyMappings, propertyMappingModel{
			PropertyName:  types.StringValue(pm.PropertyName),
			PropertyType:  types.StringValue(pm.PropertyType),
			FormFieldName: types.StringValue(pm.FormFieldName),
		})
	}
	m.RunCondition = conditionToModel(a.RunCondition)
	for _, fa := range a.FieldAttachments {
		m.FieldAttachments = append(m.FieldAttachments, fieldAttachmentModel{
			FieldName: types.StringValue(fa.FieldName),
			Condition: conditionToModel(fa.Condition),
		})
	}
	for _, sa := range a.StaticAttachments {
		m.StaticAttachments = append(m.StaticAttachments, staticAttachmentModel{
			// Source/SourceHash are provider-only (not returned by the API); the
			// form resource re-merges them from the plan after apply.
			Source:      types.StringNull(),
			SourceHash:  types.StringNull(),
			S3Key:       types.StringValue(sa.S3Key),
			FileName:    types.StringValue(sa.FileName),
			ContentType: types.StringValue(sa.ContentType),
			FileSize:    types.Int64Value(sa.FileSize),
			Condition:   conditionToModel(sa.Condition),
		})
	}
	return m
}

// stripUnresolvedAttachments removes static attachments that have no s3_key yet
// (source-based ones awaiting upload), for the initial create POST.
func stripUnresolvedAttachments(body client.FormRequest) client.FormRequest {
	for i := range body.SubmitActions {
		var kept []client.StaticEmailAttachment
		for _, sa := range body.SubmitActions[i].StaticAttachments {
			if sa.S3Key != "" {
				kept = append(kept, sa)
			}
		}
		body.SubmitActions[i].StaticAttachments = kept
	}
	return body
}

// hasSourceAttachments reports whether any static attachment needs uploading.
func hasSourceAttachments(m formResourceModel) bool {
	for _, act := range m.SubmitActions {
		for _, sa := range act.StaticAttachments {
			if optStr(sa.Source) != "" {
				return true
			}
		}
	}
	return false
}

// resolveUploads uploads any static attachment that has a `source`, filling in
// its s3_key/file_name/content_type/file_size on the plan. Attachments whose
// (source, source_hash) already match a resolved entry in prior state are reused
// rather than re-uploaded. plan attachments without a source are left untouched.
func (r *formResource) resolveUploads(ctx context.Context, formID string, plan *formResourceModel, prior *formResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	reuse := map[string]staticAttachmentModel{}
	if prior != nil {
		for _, act := range prior.SubmitActions {
			for _, sa := range act.StaticAttachments {
				if optStr(sa.Source) != "" && optStr(sa.S3Key) != "" {
					reuse[optStr(sa.Source)+"\x00"+optStr(sa.SourceHash)] = sa
				}
			}
		}
	}

	for ai := range plan.SubmitActions {
		for si := range plan.SubmitActions[ai].StaticAttachments {
			sa := &plan.SubmitActions[ai].StaticAttachments[si]
			src := optStr(sa.Source)
			if src == "" {
				continue
			}
			if ex, ok := reuse[src+"\x00"+optStr(sa.SourceHash)]; ok {
				sa.S3Key, sa.FileName, sa.ContentType, sa.FileSize = ex.S3Key, ex.FileName, ex.ContentType, ex.FileSize
				continue
			}
			content, err := os.ReadFile(src)
			if err != nil {
				diags.AddError("Error reading attachment source", err.Error())
				continue
			}
			out, err := r.client.UploadEmailAttachment(ctx, formID, filepath.Base(src), content)
			if err != nil {
				diags.AddError("Error uploading email attachment", err.Error())
				continue
			}
			sa.S3Key = types.StringValue(out.S3Key)
			sa.FileName = types.StringValue(out.FileName)
			sa.ContentType = types.StringValue(out.ContentType)
			sa.FileSize = types.Int64Value(out.FileSize)
		}
	}
	return diags
}

// attachmentSourceMap snapshots s3_key -> (source, source_hash) for the resolved
// plan, so the provider-only source fields can be re-applied after apply().
func attachmentSourceMap(m formResourceModel) map[string]staticAttachmentModel {
	out := map[string]staticAttachmentModel{}
	for _, act := range m.SubmitActions {
		for _, sa := range act.StaticAttachments {
			if optStr(sa.Source) != "" && optStr(sa.S3Key) != "" {
				out[optStr(sa.S3Key)] = sa
			}
		}
	}
	return out
}

// mergeAttachmentSources restores the provider-only source/source_hash fields on
// each static attachment by matching the API-returned s3_key.
func mergeAttachmentSources(m *formResourceModel, byKey map[string]staticAttachmentModel) {
	for ai := range m.SubmitActions {
		for si := range m.SubmitActions[ai].StaticAttachments {
			sa := &m.SubmitActions[ai].StaticAttachments[si]
			if ex, ok := byKey[optStr(sa.S3Key)]; ok {
				sa.Source = ex.Source
				sa.SourceHash = ex.SourceHash
			}
		}
	}
}

func redirectToModel(rs *client.RedirectSettings) redirectModel {
	var m redirectModel
	if rs.Success != nil {
		m.Success = []redirectBranchModel{redirectBranchToModel(rs.Success)}
	}
	if rs.Error != nil {
		m.Error = []redirectBranchModel{redirectBranchToModel(rs.Error)}
	}
	return m
}

func redirectBranchToModel(b *client.RedirectConfig) redirectBranchModel {
	return redirectBranchModel{
		Type:      types.StringValue(b.Type),
		CustomURL: strFromPtr(b.CustomURL),
		Message:   strFromPtr(b.Message),
	}
}

func valueOr(s types.String, def string) string {
	if s.IsNull() || s.IsUnknown() || s.ValueString() == "" {
		return def
	}
	return s.ValueString()
}

func valueBoolOr(b types.Bool, def bool) bool {
	if b.IsNull() || b.IsUnknown() {
		return def
	}
	return b.ValueBool()
}

func nullIfEmpty(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}

// explicitList returns a pointer to the slice, or to an empty slice when nil,
// so the field is always sent and an unset list clears the server value.
func explicitList(in []string) *[]string {
	if in == nil {
		in = []string{}
	}
	return &in
}

// boolFromResponse resolves a bool setting the API may omit. A returned value
// wins. Otherwise a known prior value is kept, and a null or unknown one
// becomes fallback.
func boolFromResponse(resp *bool, prior types.Bool, fallback types.Bool) types.Bool {
	if resp != nil {
		return types.BoolValue(*resp)
	}
	if prior.IsNull() || prior.IsUnknown() {
		return fallback
	}
	return prior
}

// listFromResponse resolves a string-list setting the API may omit. When it is
// omitted the prior value is kept (unknown becomes null). An empty returned list
// becomes null, unless the prior value is a known empty list, so both an unset
// attribute and an explicit `[]` round-trip without a diff.
func listFromResponse(resp *[]string, prior types.List) types.List {
	if resp == nil {
		if prior.IsUnknown() {
			return types.ListNull(types.StringType)
		}
		return prior
	}
	if len(*resp) == 0 {
		if !prior.IsNull() && !prior.IsUnknown() && len(prior.Elements()) == 0 {
			return prior
		}
		return types.ListNull(types.StringType)
	}
	return toStringList(*resp)
}
