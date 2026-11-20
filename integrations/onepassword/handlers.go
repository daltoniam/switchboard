package onepassword

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	op "github.com/1password/onepassword-sdk-go"
	mcp "github.com/daltoniam/switchboard"
)

// redacted replaces concealed values unless the caller asks to reveal them.
const redacted = "[REDACTED]"

const (
	defaultItemLimit = 200
	maxItemLimit     = 1000
	// maxFieldInputs bounds the custom fields one call may add or change;
	// adding fields costs time per field already on the item.
	maxFieldInputs = 100
)

// untitledSectionID is where 1Password keeps custom fields added without a
// section title.
const untitledSectionID = "add more"

type vaultView struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Type        string    `json:"type"`
	Items       uint32    `json:"items"`
	UpdatedAt   time.Time `json:"updated_at,omitzero"`
}

type itemSummary struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Category  string    `json:"category"`
	VaultID   string    `json:"vault_id"`
	URLs      []string  `json:"urls"`
	Tags      []string  `json:"tags"`
	UpdatedAt time.Time `json:"updated_at,omitzero"`
}

type itemView struct {
	ID        string      `json:"id"`
	Title     string      `json:"title"`
	Category  string      `json:"category"`
	VaultID   string      `json:"vault_id"`
	Vault     string      `json:"vault"`
	Version   uint32      `json:"version"`
	URLs      []string    `json:"urls"`
	Tags      []string    `json:"tags"`
	Notes     string      `json:"notes"`
	Fields    []fieldView `json:"fields"`
	CreatedAt time.Time   `json:"created_at,omitzero"`
	UpdatedAt time.Time   `json:"updated_at,omitzero"`
}

type fieldView struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Section   string `json:"section"`
	Type      string `json:"type"`
	Value     string `json:"value"`
	Concealed bool   `json:"concealed"`
	OTPCode   string `json:"otp_code,omitempty"`
	Reference string `json:"reference"`
}

func listVaults(ctx context.Context, s store, _ map[string]any) (*mcp.ToolResult, error) {
	vaults, err := s.ListVaults(ctx)
	if err != nil {
		return mcp.ErrResult(sdkErr(err))
	}
	out := make([]vaultView, 0, len(vaults))
	for _, v := range vaults {
		out = append(out, vaultView{ID: v.ID, Title: v.Title, Description: v.Description, Type: string(v.VaultType), Items: v.ActiveItemCount, UpdatedAt: v.UpdatedAt})
	}
	return mcp.JSONResult(out)
}

func listItems(ctx context.Context, s store, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	vaultRef := r.Str("vault")
	query := strings.ToLower(strings.TrimSpace(r.Str("query")))
	categoryRef := r.Str("category")
	limit := defaultItemLimit
	if _, ok := args["limit"]; ok {
		limit = r.Int("limit")
	}
	offset := 0
	if _, ok := args["offset"]; ok {
		offset = r.Int("offset")
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if limit < 1 || limit > maxItemLimit {
		return mcp.ErrResult(fmt.Errorf("onepassword: limit must be between 1 and %d", maxItemLimit))
	}
	if offset < 0 {
		return mcp.ErrResult(fmt.Errorf("onepassword: offset must be 0 or more"))
	}
	var category op.ItemCategory
	if categoryRef != "" {
		c, ok := parseCategory(categoryRef, listableCategories)
		if !ok {
			return mcp.ErrResult(fmt.Errorf("onepassword: unknown category %q; use one of: %s", categoryRef, categoryNames(listableCategories)))
		}
		category = c
	}
	vault, err := findVault(ctx, s, vaultRef)
	if err != nil {
		return mcp.ErrResult(err)
	}
	items, err := s.ListItems(ctx, vault.ID)
	if err != nil {
		return mcp.ErrResult(sdkErr(err))
	}
	matches := make([]itemSummary, 0, min(len(items), limit))
	total := 0
	for _, it := range items {
		if category != "" && it.Category != category {
			continue
		}
		urls := websiteURLs(it.Websites)
		if query != "" && !matchesQuery(query, it.Title, urls, it.Tags) {
			continue
		}
		total++
		if total > offset && len(matches) < limit {
			matches = append(matches, itemSummary{ID: it.ID, Title: it.Title, Category: string(it.Category), VaultID: it.VaultID, URLs: urls, Tags: orEmpty(it.Tags), UpdatedAt: it.UpdatedAt})
		}
	}
	out := map[string]any{"items": matches, "count": len(matches), "total": total}
	if next := offset + len(matches); next < total {
		out["next_offset"] = next
	}
	return mcp.JSONResult(out)
}

func getItem(ctx context.Context, s store, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	vaultRef := r.Str("vault")
	itemRef := r.Str("item")
	reveal := r.Bool("reveal")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	vault, err := findVault(ctx, s, vaultRef)
	if err != nil {
		return mcp.ErrResult(err)
	}
	item, err := findItem(ctx, s, vault, itemRef)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(viewItem(item, vault.Title, reveal))
}

func createItem(ctx context.Context, s store, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	vaultRef := r.Str("vault")
	title := strings.TrimSpace(r.Str("title"))
	categoryRef := r.Str("category")
	username := r.Str("username")
	password := r.Str("password")
	generate := r.Bool("generate_password")
	url := strings.TrimSpace(r.Str("url"))
	notes := r.Str("notes")
	tags := r.StrSlice("tags")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if title == "" {
		return mcp.ErrResult(fmt.Errorf("onepassword: title is required"))
	}
	category := op.ItemCategoryLogin
	if categoryRef != "" {
		c, ok := parseCategory(categoryRef, creatableCategories)
		if !ok {
			return mcp.ErrResult(fmt.Errorf("onepassword: unknown category %q for a new item; use one of: %s", categoryRef, categoryNames(creatableCategories)))
		}
		category = c
	}
	if err := checkLoginArgs(category, username != "", password != "", generate, url != ""); err != nil {
		return mcp.ErrResult(err)
	}
	inputs, err := parseFieldInputs(args["fields"])
	if err != nil {
		return mcp.ErrResult(err)
	}
	for i, in := range inputs {
		if in.Title == "" {
			return mcp.ErrResult(fmt.Errorf("onepassword: fields[%d] needs a title", i))
		}
	}
	vault, err := findVault(ctx, s, vaultRef)
	if err != nil {
		return mcp.ErrResult(err)
	}
	if generate {
		if password, err = s.GeneratePassword(ctx); err != nil {
			return mcp.ErrResult(sdkErr(err))
		}
	}

	item := op.Item{Category: category, VaultID: vault.ID}
	if username != "" {
		item.Fields = append(item.Fields, op.ItemField{ID: "username", Title: "username", FieldType: op.ItemFieldTypeText, Value: username})
	}
	if password != "" {
		item.Fields = append(item.Fields, op.ItemField{ID: "password", Title: "password", FieldType: op.ItemFieldTypeConcealed, Value: password})
	}
	for _, in := range inputs {
		addField(&item, in)
	}
	params := op.ItemCreateParams{Category: category, VaultID: vault.ID, Title: title, Fields: item.Fields, Sections: item.Sections, Tags: tags}
	if notes != "" {
		params.Notes = &notes
	}
	if url != "" {
		params.Websites = []op.Website{{URL: url, Label: "website", AutofillBehavior: op.AutofillBehaviorAnywhereOnWebsite}}
	}
	created, err := s.CreateItem(ctx, params)
	if err != nil {
		return mcp.ErrResult(sdkErr(err))
	}
	return mcp.JSONResult(viewItem(created, vault.Title, false))
}

func updateItem(ctx context.Context, s store, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	vaultRef := r.Str("vault")
	itemRef := r.Str("item")
	title := strings.TrimSpace(r.Str("title"))
	username := r.Str("username")
	password := r.Str("password")
	generate := r.Bool("generate_password")
	notes := r.Str("notes")
	tags := r.StrSlice("tags")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	_, setNotes := args["notes"]
	_, setTags := args["tags"]
	inputs, err := parseFieldInputs(args["fields"])
	if err != nil {
		return mcp.ErrResult(err)
	}
	for i, in := range inputs {
		if in.ID == "" && in.Title == "" {
			return mcp.ErrResult(fmt.Errorf("onepassword: fields[%d] needs an id or a title", i))
		}
	}
	if title == "" && username == "" && password == "" && !generate && !setNotes && !setTags && len(inputs) == 0 {
		return mcp.ErrResult(fmt.Errorf("onepassword: nothing to update; pass title, username, password, generate_password, notes, tags or fields"))
	}
	vault, err := findVault(ctx, s, vaultRef)
	if err != nil {
		return mcp.ErrResult(err)
	}
	item, err := findItem(ctx, s, vault, itemRef)
	if err != nil {
		return mcp.ErrResult(err)
	}
	if err := checkLoginArgs(item.Category, username != "", password != "", generate, false); err != nil {
		return mcp.ErrResult(err)
	}
	// Check every field before generating a password, so a bad request
	// changes nothing.
	targets := make([]int, len(inputs))
	for i, in := range inputs {
		idx, err := matchField(item, in)
		if err != nil {
			return mcp.ErrResult(err)
		}
		targets[i] = idx
	}
	if generate {
		if password, err = s.GeneratePassword(ctx); err != nil {
			return mcp.ErrResult(sdkErr(err))
		}
	}

	if title != "" {
		item.Title = title
	}
	if setNotes {
		item.Notes = notes
	}
	if setTags {
		item.Tags = tags
	}
	if username != "" {
		setBuiltIn(&item, "username", op.ItemFieldTypeText, username)
	}
	if password != "" {
		setBuiltIn(&item, "password", op.ItemFieldTypeConcealed, password)
	}
	for i, in := range inputs {
		if targets[i] < 0 {
			addField(&item, in)
			continue
		}
		f := &item.Fields[targets[i]]
		f.Value = in.Value
		if in.Type != "" {
			f.FieldType = in.fieldType
		}
	}
	updated, err := s.PutItem(ctx, item)
	if err != nil {
		return mcp.ErrResult(sdkErr(err))
	}
	return mcp.JSONResult(viewItem(updated, vault.Title, false))
}

func resolveSecret(ctx context.Context, s store, args map[string]any) (*mcp.ToolResult, error) {
	reference, err := mcp.ArgStr(args, "reference")
	if err != nil {
		return mcp.ErrResult(err)
	}
	reference = strings.TrimSpace(reference)
	if reference == "" {
		return mcp.ErrResult(fmt.Errorf("onepassword: reference is required"))
	}
	if !strings.HasPrefix(reference, "op://") {
		return mcp.ErrResult(fmt.Errorf("onepassword: reference must start with op://, such as op://vault/item/field"))
	}
	value, err := s.ResolveSecret(ctx, reference)
	if err != nil {
		return mcp.ErrResult(sdkErr(err))
	}
	return mcp.JSONResult(map[string]string{"reference": reference, "value": value})
}

// findVault matches a vault by id, then by title in any case.
func findVault(ctx context.Context, s store, ref string) (op.VaultOverview, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return op.VaultOverview{}, fmt.Errorf("onepassword: vault is required")
	}
	vaults, err := s.ListVaults(ctx)
	if err != nil {
		return op.VaultOverview{}, sdkErr(err)
	}
	var matches []op.VaultOverview
	for _, v := range vaults {
		if v.ID == ref {
			return v, nil
		}
		if strings.EqualFold(v.Title, ref) {
			matches = append(matches, v)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		titles := make([]string, 0, len(vaults))
		for _, v := range vaults {
			titles = append(titles, v.Title)
		}
		return op.VaultOverview{}, fmt.Errorf("onepassword: vault %q not found; vaults: %s", ref, strings.Join(titles, ", "))
	default:
		ids := make([]string, 0, len(matches))
		for _, v := range matches {
			ids = append(ids, v.ID)
		}
		return op.VaultOverview{}, fmt.Errorf("onepassword: vault %q matches %d vaults; pass one of these ids: %s", ref, len(matches), strings.Join(ids, ", "))
	}
}

// findItem matches an item in the vault by id, then by title in any case,
// and reads it in full.
func findItem(ctx context.Context, s store, vault op.VaultOverview, ref string) (op.Item, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return op.Item{}, fmt.Errorf("onepassword: item is required")
	}
	items, err := s.ListItems(ctx, vault.ID)
	if err != nil {
		return op.Item{}, sdkErr(err)
	}
	var matches []op.ItemOverview
	for _, it := range items {
		if it.ID == ref {
			matches = []op.ItemOverview{it}
			break
		}
		if strings.EqualFold(it.Title, ref) {
			matches = append(matches, it)
		}
	}
	switch len(matches) {
	case 0:
		return op.Item{}, fmt.Errorf("onepassword: item %q not found in vault %s; use onepassword_list_items to find it", ref, vault.Title)
	case 1:
	default:
		ids := make([]string, 0, len(matches))
		for _, it := range matches {
			ids = append(ids, it.ID)
		}
		return op.Item{}, fmt.Errorf("onepassword: item %q matches %d items in vault %s; pass one of these ids: %s", ref, len(matches), vault.Title, strings.Join(ids, ", "))
	}
	item, err := s.GetItem(ctx, vault.ID, matches[0].ID)
	if err != nil {
		return op.Item{}, sdkErr(err)
	}
	return item, nil
}

func viewItem(it op.Item, vaultTitle string, reveal bool) itemView {
	sections := map[string]string{}
	for _, s := range it.Sections {
		sections[s.ID] = s.Title
	}
	fields := make([]fieldView, 0, len(it.Fields))
	for _, f := range it.Fields {
		fv := fieldView{ID: f.ID, Title: f.Title, Type: string(f.FieldType), Value: f.Value, Reference: fieldReference(it, f)}
		if f.SectionID != nil {
			fv.Section = sections[*f.SectionID]
		}
		if concealedTypes[f.FieldType] {
			conceal(&fv, f, reveal)
		}
		fields = append(fields, fv)
	}
	return itemView{
		ID: it.ID, Title: it.Title, Category: string(it.Category), VaultID: it.VaultID, Vault: vaultTitle, Version: it.Version,
		URLs: websiteURLs(it.Websites), Tags: orEmpty(it.Tags), Notes: it.Notes, Fields: fields, CreatedAt: it.CreatedAt, UpdatedAt: it.UpdatedAt,
	}
}

// conceal marks a secret field and, unless revealing, replaces its value
// and one-time code with the redaction marker.
func conceal(fv *fieldView, f op.ItemField, reveal bool) {
	fv.Concealed = true
	if !reveal {
		fv.Value = redacted
		if f.FieldType == op.ItemFieldTypeTOTP {
			fv.OTPCode = redacted
		}
		return
	}
	if f.Details == nil {
		return
	}
	if otp := f.Details.OTP(); otp != nil && otp.Code != nil {
		fv.OTPCode = *otp.Code
	}
}

// fieldReference builds the op:// secret reference for a field from ids,
// which stay valid when titles change.
func fieldReference(it op.Item, f op.ItemField) string {
	parts := []string{it.VaultID, it.ID}
	if f.SectionID != nil && *f.SectionID != "" {
		parts = append(parts, *f.SectionID)
	}
	return "op://" + strings.Join(append(parts, f.ID), "/")
}

// concealedTypes are field types whose values are secrets.
var concealedTypes = map[op.ItemFieldType]bool{
	op.ItemFieldTypeConcealed:        true,
	op.ItemFieldTypeCreditCardNumber: true,
	op.ItemFieldTypeTOTP:             true,
	op.ItemFieldTypeSSHKey:           true,
}

func websiteURLs(sites []op.Website) []string {
	urls := make([]string, 0, len(sites))
	for _, w := range sites {
		urls = append(urls, w.URL)
	}
	return urls
}

// orEmpty keeps list fields as [] rather than null in responses.
func orEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func matchesQuery(query, title string, urls, tags []string) bool {
	if strings.Contains(strings.ToLower(title), query) {
		return true
	}
	for _, s := range slices.Concat(urls, tags) {
		if strings.Contains(strings.ToLower(s), query) {
			return true
		}
	}
	return false
}

// creatableCategories excludes Document, which needs a file upload.
var creatableCategories = []op.ItemCategory{
	op.ItemCategoryLogin, op.ItemCategoryPassword, op.ItemCategorySecureNote, op.ItemCategoryAPICredentials,
	op.ItemCategoryDatabase, op.ItemCategoryServer, op.ItemCategoryCreditCard, op.ItemCategoryBankAccount,
	op.ItemCategoryIdentity, op.ItemCategorySoftwareLicense, op.ItemCategorySSHKey, op.ItemCategoryEmail,
	op.ItemCategoryMembership, op.ItemCategoryRouter, op.ItemCategoryCryptoWallet, op.ItemCategoryDriverLicense,
	op.ItemCategoryMedicalRecord, op.ItemCategoryOutdoorLicense, op.ItemCategoryPassport, op.ItemCategoryRewards,
	op.ItemCategorySocialSecurityNumber, op.ItemCategoryPerson,
}

var listableCategories = append(slices.Clone(creatableCategories), op.ItemCategoryDocument)

// parseCategory accepts a category in any case, with or without spaces,
// dashes or underscores ("secure note", "api_credentials").
func parseCategory(ref string, allowed []op.ItemCategory) (op.ItemCategory, bool) {
	want := squash(ref)
	for _, c := range allowed {
		if squash(string(c)) == want {
			return c, true
		}
	}
	return "", false
}

func categoryNames(cs []op.ItemCategory) string {
	names := make([]string, 0, len(cs))
	for _, c := range cs {
		names = append(names, string(c))
	}
	return strings.Join(names, ", ")
}

func squash(s string) string {
	return strings.NewReplacer(" ", "", "_", "", "-", "").Replace(strings.ToLower(strings.TrimSpace(s)))
}

// checkLoginArgs rejects login-only arguments on categories without the
// matching built-in fields.
func checkLoginArgs(c op.ItemCategory, username, password, generate, url bool) error {
	isLogin := c == op.ItemCategoryLogin
	hasPassword := isLogin || c == op.ItemCategoryPassword
	switch {
	case username && !isLogin:
		return fmt.Errorf("onepassword: username applies to Login items; use fields for a %s item", c)
	case (password || generate) && !hasPassword:
		return fmt.Errorf("onepassword: password applies to Login and Password items; use fields with type concealed for a %s item", c)
	case url && !hasPassword:
		return fmt.Errorf("onepassword: url applies to Login and Password items; use fields with type url for a %s item", c)
	case password && generate:
		return fmt.Errorf("onepassword: pass either password or generate_password, not both")
	}
	return nil
}

// fieldInput is one entry of the fields argument.
type fieldInput struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Value   string `json:"value"`
	Type    string `json:"type"`
	Section string `json:"section"`

	fieldType op.ItemFieldType
}

var fieldTypes = map[string]op.ItemFieldType{
	"text":      op.ItemFieldTypeText,
	"string":    op.ItemFieldTypeText,
	"concealed": op.ItemFieldTypeConcealed,
	"password":  op.ItemFieldTypeConcealed,
	"secret":    op.ItemFieldTypeConcealed,
	"url":       op.ItemFieldTypeURL,
	"email":     op.ItemFieldTypeEmail,
	"phone":     op.ItemFieldTypePhone,
	"date":      op.ItemFieldTypeDate,
	"totp":      op.ItemFieldTypeTOTP,
	"otp":       op.ItemFieldTypeTOTP,
}

// parseFieldInputs accepts the fields argument as an array of objects or as
// a JSON string of one.
func parseFieldInputs(raw any) ([]fieldInput, error) {
	if raw == nil {
		return nil, nil
	}
	var data []byte
	switch v := raw.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return nil, nil
		}
		data = []byte(v)
	case []any:
		var err error
		if data, err = json.Marshal(v); err != nil {
			return nil, fmt.Errorf("onepassword: fields must be an array of objects")
		}
	default:
		return nil, fmt.Errorf("onepassword: fields must be an array of objects")
	}
	var inputs []fieldInput
	if err := json.Unmarshal(data, &inputs); err != nil {
		return nil, fmt.Errorf("onepassword: fields must be an array of objects with string title, value, type and section")
	}
	if len(inputs) > maxFieldInputs {
		return nil, fmt.Errorf("onepassword: at most %d fields per call; split the change", maxFieldInputs)
	}
	for i := range inputs {
		in := &inputs[i]
		in.ID = strings.TrimSpace(in.ID)
		in.Title = strings.TrimSpace(in.Title)
		in.Section = strings.TrimSpace(in.Section)
		in.Type = strings.ToLower(strings.TrimSpace(in.Type))
		in.fieldType = op.ItemFieldTypeText
		if in.Type != "" {
			t, ok := fieldTypes[in.Type]
			if !ok {
				return nil, fmt.Errorf("onepassword: fields[%d]: unknown field type %q; use text, concealed, url, email, phone, date or totp", i, in.Type)
			}
			in.fieldType = t
		}
	}
	return inputs, nil
}

// matchField finds the field an input changes: by id, else by title (any
// case) within the named section. It returns -1 when the input adds a field.
func matchField(it op.Item, in fieldInput) (int, error) {
	sections := map[string]string{}
	for _, s := range it.Sections {
		sections[s.ID] = s.Title
	}
	found := -1
	for i, f := range it.Fields {
		if in.ID != "" && f.ID != in.ID {
			continue
		}
		if in.ID == "" && !strings.EqualFold(f.Title, in.Title) {
			continue
		}
		if in.Section != "" && (f.SectionID == nil || !strings.EqualFold(sections[*f.SectionID], in.Section)) {
			continue
		}
		if found >= 0 {
			if in.ID != "" {
				return -1, fmt.Errorf("onepassword: more than one field has id %q in item %s; pass its section", in.ID, it.Title)
			}
			return -1, fmt.Errorf("onepassword: more than one field is titled %q in item %s; pass its id or section", in.Title, it.Title)
		}
		found = i
	}
	if found < 0 && in.ID != "" {
		return -1, fmt.Errorf("onepassword: no field with id %q in item %s; give a title to add a new field", in.ID, it.Title)
	}
	return found, nil
}

// addField appends a custom field, in the named section or the untitled
// one, creating the section when the item lacks it.
func addField(it *op.Item, in fieldInput) {
	sectionID := ensureSection(it, in.Section)
	// The built-in ids stay reserved even when the item lacks those fields,
	// so a later username or password update never lands in a custom field.
	taken := map[string]bool{"username": true, "password": true}
	for _, f := range it.Fields {
		taken[f.ID] = true
	}
	it.Fields = append(it.Fields, op.ItemField{ID: uniqueID(slug(in.Title), taken), Title: in.Title, SectionID: &sectionID, FieldType: in.fieldType, Value: in.Value})
}

func ensureSection(it *op.Item, title string) string {
	for _, s := range it.Sections {
		if strings.EqualFold(s.Title, title) {
			return s.ID
		}
	}
	taken := map[string]bool{}
	for _, s := range it.Sections {
		taken[s.ID] = true
	}
	// The untitled section's usual id may belong to a section since renamed.
	base := untitledSectionID
	if title != "" {
		base = slug(title)
	}
	id := uniqueID(base, taken)
	it.Sections = append(it.Sections, op.ItemSection{ID: id, Title: title})
	return id
}

// setBuiltIn sets a login's built-in username or password field, adding it
// when missing. Built-in fields sit outside every section; a custom field
// that shares the id is left alone. The field type is always reapplied so a
// password is stored concealed even if the field was saved as text.
func setBuiltIn(it *op.Item, id string, t op.ItemFieldType, value string) {
	for i := range it.Fields {
		f := &it.Fields[i]
		if f.ID == id && (f.SectionID == nil || *f.SectionID == "") {
			f.Value = value
			f.FieldType = t
			return
		}
	}
	it.Fields = append(it.Fields, op.ItemField{ID: id, Title: id, FieldType: t, Value: value})
}

func slug(title string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(title) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	s := strings.TrimSuffix(b.String(), "-")
	if s == "" {
		return "field"
	}
	return s
}

func uniqueID(base string, taken map[string]bool) string {
	id := base
	for n := 2; taken[id]; n++ {
		id = fmt.Sprintf("%s-%d", base, n)
	}
	return id
}
