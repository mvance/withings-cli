// Package measures handles Withings measure endpoints.
package measures

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/mreimbold/withings-cli/internal/app"
	"github.com/mreimbold/withings-cli/internal/errs"
	"github.com/mreimbold/withings-cli/internal/filters"
	"github.com/mreimbold/withings-cli/internal/output"
	"github.com/mreimbold/withings-cli/internal/params"
	"github.com/mreimbold/withings-cli/internal/withings"
)

const (
	serviceName      = "measure"
	actionGet        = "getmeas"
	typeParam        = "meastypes"
	categoryParam    = "category"
	startDateParam   = "startdate"
	endDateParam     = "enddate"
	lastUpdateParam  = "lastupdate"
	userIDParam      = "userid"
	limitParam       = "limit"
	offsetParam      = "offset"
	categoryReal     = "1"
	categoryGoal     = "2"
	categoryRealText = "real"
	categoryGoalText = "goal"
	typeDelimiter    = ","
	aliasBodyWeight  = "bodyweight"
	aliasTemperature = "temperature"
	numberBase10     = 10
	zeroString       = "0"
	unitBase         = "1"
	unitExponent     = "1e"
	negativeSign     = "-"
	decimalSeparator = "."
	rowsHeaderCount  = 1
	tableMinWidth    = 0
	tableTabWidth    = 0
	tablePadding     = 2
	tablePadChar     = ' '
	tableFlags       = 0
	scalePad         = 1
	defaultInt       = 0
	defaultInt64     = 0
	emptyString      = ""
	kgToLb           = 2.20462262
	unitImperial     = "imperial"
)

var (
	errInvalidMeasureType     = errors.New("invalid measure type")
	errInvalidMeasureCategory = errors.New("invalid measure category")
	errInvalidStartTime       = errs.ErrInvalidStartTime
	errInvalidEndTime         = errs.ErrInvalidEndTime
	errInvalidLastUpdate      = errs.ErrInvalidLastUpdate
	errLastUpdateConflict     = errs.ErrLastUpdateConflict
	errMeasureTypesMissing    = errors.New("measure type list is empty")
)

// Options captures measure query parameters.
type Options struct {
	TimeRange  params.TimeRange
	Pagination params.Pagination
	User       params.User
	LastUpdate params.LastUpdate
	Types      string
	Category   string
	Units      string
}

// Run fetches body measures and writes output.
func Run(
	ctx context.Context,
	opts Options,
	appOpts app.Options,
	accessToken string,
) error {
	values, err := buildParams(opts)
	if err != nil {
		return app.NewExitError(app.ExitCodeUsage, err)
	}

	req, _, err := withings.BuildRequest(
		ctx,
		withings.APIBaseURL(appOpts.BaseURL, appOpts.Cloud),
		serviceName,
		actionGet,
		accessToken,
		values,
	)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	//nolint:bodyclose // ReadPayload closes the response body.
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return app.NewExitError(app.ExitCodeNetwork, err)
	}

	payload, err := withings.ReadPayload(resp)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	return writeResponse(opts, appOpts, payload)
}

func buildParams(opts Options) (url.Values, error) {
	values := url.Values{}

	switch opts.Units {
	case "metric", "imperial", "":
		// Valid
	default:
		return nil, fmt.Errorf("invalid units %q: must be metric or imperial", opts.Units)
	}

	err := applyTypes(&values, opts.Types)
	if err != nil {
		return nil, err
	}

	err = applyCategory(&values, opts.Category)
	if err != nil {
		return nil, err
	}

	err = applyTimeFilters(&values, opts.TimeRange, opts.LastUpdate)
	if err != nil {
		return nil, err
	}

	applyUser(&values, opts.User)
	applyPagination(&values, opts.Pagination)

	return values, nil
}

func applyTypes(values *url.Values, raw string) error {
	if raw == emptyString {
		return nil
	}

	types, err := parseTypes(raw)
	if err != nil {
		return err
	}

	if types == emptyString {
		return errMeasureTypesMissing
	}

	values.Set(typeParam, types)

	return nil
}

func applyCategory(values *url.Values, raw string) error {
	if raw == emptyString {
		return nil
	}

	category, err := parseCategory(raw)
	if err != nil {
		return err
	}

	values.Set(categoryParam, category)

	return nil
}

func applyTimeFilters(
	values *url.Values,
	timeRange params.TimeRange,
	lastUpdate params.LastUpdate,
) error {
	err := filters.ApplyLastUpdateFilter(
		values,
		lastUpdateParam,
		lastUpdate,
		params.Date{Date: emptyString},
		timeRange,
		errInvalidLastUpdate,
		errLastUpdateConflict,
	)
	if err != nil {
		return fmt.Errorf("apply last-update filter: %w", err)
	}

	err = applyTimeValue(
		values,
		startDateParam,
		timeRange.Start,
		errInvalidStartTime,
	)
	if err != nil {
		return err
	}

	return applyTimeValue(
		values,
		endDateParam,
		timeRange.End,
		errInvalidEndTime,
	)
}

func applyTimeValue(
	values *url.Values,
	key string,
	raw string,
	errInvalid error,
) error {
	if raw == emptyString {
		return nil
	}

	epoch, err := filters.ParseEpoch(raw)
	if err != nil {
		return fmt.Errorf("%w: %w", errInvalid, err)
	}

	values.Set(key, strconv.FormatInt(epoch, numberBase10))

	return nil
}

func applyUser(values *url.Values, user params.User) {
	if user.UserID == emptyString {
		return
	}

	values.Set(userIDParam, user.UserID)
}

func applyPagination(values *url.Values, pagination params.Pagination) {
	if pagination.Limit > defaultInt {
		values.Set(limitParam, strconv.Itoa(pagination.Limit))
	}

	if pagination.Offset > defaultInt {
		values.Set(offsetParam, strconv.Itoa(pagination.Offset))
	}
}

func parseCategory(value string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))

	switch normalized {
	case categoryReal, categoryRealText:
		return categoryReal, nil
	case categoryGoal, categoryGoalText:
		return categoryGoal, nil
	default:
		return emptyString, fmt.Errorf(
			"%w: %q",
			errInvalidMeasureCategory,
			value,
		)
	}
}

func parseTypes(value string) (string, error) {
	parts := strings.Split(value, typeDelimiter)
	types := make([]string, defaultInt, len(parts))
	seen := map[string]bool{}

	for _, raw := range parts {
		trimmed := strings.ToLower(strings.TrimSpace(raw))
		if trimmed == emptyString {
			continue
		}

		resolved, err := resolveType(trimmed)
		if err != nil {
			return emptyString, err
		}

		if seen[resolved] {
			continue
		}

		seen[resolved] = true
		types = append(types, resolved)
	}

	return strings.Join(types, typeDelimiter), nil
}

func resolveType(value string) (string, error) {
	if isDigits(value) {
		return value, nil
	}

	mapped, ok := typeMap[value]
	if !ok {
		return emptyString, fmt.Errorf("%w: %q", errInvalidMeasureType, value)
	}

	return mapped, nil
}

func isDigits(value string) bool {
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}

	return value != emptyString
}

type response struct {
	Status int    `json:"status"`
	Body   body   `json:"body"`
	Error  string `json:"error"`
	Detail string `json:"detail"`
}

type body struct {
	UpdateTime    int64   `json:"updatetime"`
	Timezone      string  `json:"timezone"`
	MeasureGroups []group `json:"measuregrps"`
}

type group struct {
	GroupID  int64  `json:"grpid"`
	Attrib   int    `json:"attrib"`
	Date     int64  `json:"date"`
	Category int    `json:"category"`
	Measures []item `json:"measures"`
}

type item struct {
	Type  int   `json:"type"`
	Value int64 `json:"value"`
	Unit  int   `json:"unit"`
}

type row struct {
	Time     string
	Type     string
	Value    string
	Unit     string
	Category string
}

//nolint:gochecknoglobals // Static lookup table for CLI aliases.
var typeMap = map[string]string{
	"weight":              "1",
	"fat_free_mass":       "5",
	"fat_ratio":           "6",
	"fat_mass":            "8",
	"fat_mass_weight":     "8",
	"bp_dia":              "9",
	"bp_sys":              "10",
	"heart_rate":          "11",
	"temp":                "12",
	"spo2":                "54",
	"body_temp":           "71",
	"skin_temp":           "73",
	"muscle_mass":         "76",
	"hydration":           "77",
	"bone_mass":           "88",
	"pulse_wave_velocity": "91",
	aliasBodyWeight:       "1",
	aliasTemperature:      "12",
}

//nolint:gochecknoglobals // Static lookup tables for measure metadata.
var (
	typeNameByID = map[string]string{
		"1":  "weight",
		"5":  "fat_free_mass",
		"6":  "fat_ratio",
		"8":  "fat_mass",
		"9":  "bp_dia",
		"10": "bp_sys",
		"11": "heart_rate",
		"12": "temp",
		"54": "spo2",
		"71": "body_temp",
		"73": "skin_temp",
		"76": "muscle_mass",
		"77": "hydration",
		"88": "bone_mass",
		"91": "pulse_wave_velocity",
	}
	unitByTypeID = map[string]string{
		"1":  "kg",
		"5":  "kg",
		"6":  "%",
		"8":  "kg",
		"9":  "mmHg",
		"10": "mmHg",
		"11": "bpm",
		"12": "C",
		"54": "%",
		"71": "C",
		"73": "C",
		"76": "kg",
		"77": "%",
		"88": "kg",
		"91": "m/s",
	}
)

func writeResponse(opts Options, appOpts app.Options, payload []byte) error {
	decoded, err := decodeResponse(payload)
	if err != nil {
		return err
	}

	return writeBody(opts, appOpts, decoded.Body)
}

func writeBody(opts Options, appOpts app.Options, body body) error {
	if appOpts.Quiet {
		return nil
	}

	if appOpts.JSON {
		return writeJSONOutput(opts, appOpts, body)
	}

	rows := buildRows(opts, body)

	if appOpts.Plain {
		return writePlainOutput(rows)
	}

	return writeTableOutput(rows)
}

func writeJSONOutput(opts Options, appOpts app.Options, body body) error {
	err := output.WriteRawJSON(appOpts, body)
	if err != nil {
		return fmt.Errorf("write json output: %w", err)
	}

	return nil
}

func writePlainOutput(rows []row) error {
	err := output.WriteLines(formatLines(rows))
	if err != nil {
		return fmt.Errorf("write plain output: %w", err)
	}

	return nil
}

func writeTableOutput(rows []row) error {
	table, err := formatTable(rows)
	if err != nil {
		return err
	}

	err = output.WriteLine(table)
	if err != nil {
		return fmt.Errorf("write table output: %w", err)
	}

	return nil
}

func decodeResponse(payload []byte) (response, error) {
	var decoded response

	err := json.Unmarshal(payload, &decoded)
	if err != nil {
		return response{}, app.NewExitError(
			app.ExitCodeFailure,
			fmt.Errorf("decode api response: %w", err),
		)
	}

	if decoded.Status != withings.StatusOK {
		message := decoded.Error
		if message == emptyString {
			message = decoded.Detail
		}

		if message == emptyString {
			message = strings.TrimSpace(string(payload))
		}

		return response{}, app.NewExitError(
			app.ExitCodeAPI,
			fmt.Errorf("%w: %d: %s", withings.ErrAPI, decoded.Status, message),
		)
	}

	return decoded, nil
}

func buildRows(opts Options, body body) []row {
	location := measureLocation(body.Timezone)
	rows := make([]row, defaultInt, len(body.MeasureGroups))

	for _, group := range body.MeasureGroups {
		timestamp := formatTime(group.Date, location)
		category := formatCategory(group.Category)

		for _, item := range group.Measures {
			typeID := strconv.Itoa(item.Type)
			val, unit := formatMeasure(opts, typeID, item.Value, item.Unit)
			rows = append(rows, row{
				Time:     timestamp,
				Type:     formatType(typeID),
				Value:    val,
				Unit:     unit,
				Category: category,
			})
		}
	}

	return rows
}

func formatMeasure(
	opts Options,
	typeID string,
	value int64,
	unit int,
) (string, string) {
	if opts.Units == unitImperial && unitByTypeID[typeID] == "kg" {
		fValue := float64(value) * math.Pow10(unit)
		lbValue := fValue * kgToLb

		return strconv.FormatFloat(lbValue, 'f', 2, 64), "lb"
	}

	return formatScaledValue(value, unit), formatUnit(typeID, unit)
}

func measureLocation(timezone string) *time.Location {
	if timezone == emptyString {
		return time.UTC
	}

	location, err := time.LoadLocation(timezone)
	if err != nil {
		return time.UTC
	}

	return location
}

func formatTime(epoch int64, location *time.Location) string {
	if epoch == defaultInt64 {
		return emptyString
	}

	return time.Unix(epoch, defaultInt64).In(location).Format(time.RFC3339)
}

func formatCategory(category int) string {
	switch strconv.Itoa(category) {
	case categoryReal:
		return categoryRealText
	case categoryGoal:
		return categoryGoalText
	default:
		return strconv.Itoa(category)
	}
}

func formatType(typeID string) string {
	if name, ok := typeNameByID[typeID]; ok {
		return name
	}

	return typeID
}

func formatUnit(typeID string, unit int) string {
	if label, ok := unitByTypeID[typeID]; ok {
		return label
	}

	return formatUnitPower(unit)
}

func formatUnitPower(unit int) string {
	if unit == defaultInt {
		return unitBase
	}

	return unitExponent + strconv.Itoa(unit)
}

func formatScaledValue(value int64, unit int) string {
	if unit == defaultInt {
		return strconv.FormatInt(value, numberBase10)
	}

	scaled := value
	sign := emptyString

	if scaled < defaultInt64 {
		sign = negativeSign
		scaled = -scaled
	}

	digits := strconv.FormatInt(scaled, numberBase10)

	if unit > defaultInt {
		return sign + digits + strings.Repeat(zeroString, unit)
	}

	scale := -unit
	if len(digits) <= scale {
		digits = strings.Repeat(
			zeroString,
			scale-len(digits)+scalePad,
		) + digits
	}

	point := len(digits) - scale
	whole := digits[:point]
	frac := strings.TrimRight(digits[point:], zeroString)

	if frac == emptyString {
		return sign + whole
	}

	return sign + whole + decimalSeparator + frac
}

func formatTable(rows []row) (string, error) {
	var buffer bytes.Buffer

	writer := tabwriter.NewWriter(
		&buffer,
		tableMinWidth,
		tableTabWidth,
		tablePadding,
		tablePadChar,
		tableFlags,
	)
	_, _ = fmt.Fprintln(writer, "Time\tType\tValue\tUnit\tCategory")

	for _, row := range rows {
		_, _ = fmt.Fprintf(
			writer,
			"%s\t%s\t%s\t%s\t%s\n",
			row.Time,
			row.Type,
			row.Value,
			row.Unit,
			row.Category,
		)
	}

	err := writer.Flush()
	if err != nil {
		return emptyString, fmt.Errorf("render measures table: %w", err)
	}

	return strings.TrimRight(buffer.String(), "\n"), nil
}

func formatLines(rows []row) []string {
	lines := make([]string, defaultInt, len(rows)+rowsHeaderCount)
	lines = append(lines, "time\ttype\tvalue\tunit\tcategory")

	for _, row := range rows {
		lines = append(lines, strings.Join([]string{
			row.Time,
			row.Type,
			row.Value,
			row.Unit,
			row.Category,
		}, "\t"))
	}

	return lines
}
