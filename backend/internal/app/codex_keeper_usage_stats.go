package app

import (
	"context"
	"database/sql"
	"log"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	keeperUsageStatsRunHour   = 2
	keeperUsageStatsRunMinute = 20
)

type KeeperUsageStatsRunner struct {
	app  *App
	mu   sync.Mutex
	stop chan struct{}
	done chan struct{}
}

type keeperUsageStatsMetric struct {
	Records             int
	SuccessRecords      int
	FailedRecords       int
	InputTokens         int
	OutputTokens        int
	CachedTokens        int
	CacheReadTokens     int
	CacheCreationTokens int
	ReasoningTokens     int
	TotalTokens         int
	EstimatedCostUSD    float64
	UnpricedRecords     int
	FirstRequestAt      *time.Time
	LastRequestAt       *time.Time
}

type keeperUsageStatsPeriod struct {
	AuthName    string
	Email       *string
	AccountType *string
	PeriodType  string
	PeriodStart time.Time
	PeriodEnd   time.Time
	Metric      keeperUsageStatsMetric
	GeneratedAt time.Time
}

type keeperUsageStatsMetricResponse struct {
	Records             int     `json:"records"`
	SuccessRecords      int     `json:"success_records"`
	FailedRecords       int     `json:"failed_records"`
	InputTokens         int     `json:"input_tokens"`
	OutputTokens        int     `json:"output_tokens"`
	CachedTokens        int     `json:"cached_tokens"`
	CacheReadTokens     int     `json:"cache_read_tokens"`
	CacheCreationTokens int     `json:"cache_creation_tokens"`
	ReasoningTokens     int     `json:"reasoning_tokens"`
	TotalTokens         int     `json:"total_tokens"`
	EstimatedCostUSD    float64 `json:"estimated_cost_usd"`
	UnpricedRecords     int     `json:"unpriced_records"`
	FirstRequestAt      *string `json:"first_request_at"`
	LastRequestAt       *string `json:"last_request_at"`
}

type keeperUsageStatsSummaryResponse struct {
	AliveDays       int                            `json:"alive_days"`
	AliveWeeks      int                            `json:"alive_weeks"`
	ActiveDays      int                            `json:"active_days"`
	ActiveWeeks     int                            `json:"active_weeks"`
	FirstRequestAt  *string                        `json:"first_request_at"`
	LastRequestAt   *string                        `json:"last_request_at"`
	LastGeneratedAt *string                        `json:"last_generated_at"`
	Today           keeperUsageStatsMetricResponse `json:"today"`
	Yesterday       keeperUsageStatsMetricResponse `json:"yesterday"`
	ThisWeek        keeperUsageStatsMetricResponse `json:"this_week"`
	LastWeek        keeperUsageStatsMetricResponse `json:"last_week"`
	TwoWeeksAgo     keeperUsageStatsMetricResponse `json:"two_weeks_ago"`
	AllTime         keeperUsageStatsMetricResponse `json:"all_time"`
}

type keeperUsageStatsPeriodResponse struct {
	PeriodType          string  `json:"period_type"`
	PeriodStart         string  `json:"period_start"`
	PeriodEnd           string  `json:"period_end"`
	Label               string  `json:"label"`
	Records             int     `json:"records"`
	SuccessRecords      int     `json:"success_records"`
	FailedRecords       int     `json:"failed_records"`
	InputTokens         int     `json:"input_tokens"`
	OutputTokens        int     `json:"output_tokens"`
	CachedTokens        int     `json:"cached_tokens"`
	CacheReadTokens     int     `json:"cache_read_tokens"`
	CacheCreationTokens int     `json:"cache_creation_tokens"`
	ReasoningTokens     int     `json:"reasoning_tokens"`
	TotalTokens         int     `json:"total_tokens"`
	EstimatedCostUSD    float64 `json:"estimated_cost_usd"`
	UnpricedRecords     int     `json:"unpriced_records"`
	FirstRequestAt      *string `json:"first_request_at"`
	LastRequestAt       *string `json:"last_request_at"`
	GeneratedAt         string  `json:"generated_at"`
}

type keeperAccountUsageStatsResponse struct {
	AuthName    string                           `json:"auth_name"`
	Email       *string                          `json:"email"`
	AccountType *string                          `json:"account_type"`
	Summary     keeperUsageStatsSummaryResponse  `json:"summary"`
	Daily       []keeperUsageStatsPeriodResponse `json:"daily"`
	Weekly      []keeperUsageStatsPeriodResponse `json:"weekly"`
}

func NewKeeperUsageStatsRunner(app *App) *KeeperUsageStatsRunner {
	return &KeeperUsageStatsRunner{app: app}
}

func (r *KeeperUsageStatsRunner) Start() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.done != nil {
		select {
		case <-r.done:
		default:
			return
		}
	}
	r.stop = make(chan struct{})
	r.done = make(chan struct{})
	go r.loop()
}

func (r *KeeperUsageStatsRunner) Stop() {
	r.mu.Lock()
	stop := r.stop
	done := r.done
	if stop == nil || done == nil {
		r.mu.Unlock()
		return
	}
	select {
	case <-stop:
	default:
		close(stop)
	}
	r.mu.Unlock()
	<-done
}

func (r *KeeperUsageStatsRunner) loop() {
	defer func() {
		r.mu.Lock()
		if r.done != nil {
			close(r.done)
		}
		r.mu.Unlock()
	}()

	r.runOnce()
	for {
		delay := keeperUsageStatsNextDelay(time.Now().In(appTimeLocation))
		timer := time.NewTimer(delay)
		select {
		case <-r.stop:
			timer.Stop()
			return
		case <-timer.C:
			r.runOnce()
		}
	}
}

func (r *KeeperUsageStatsRunner) runOnce() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := r.app.refreshKeeperAccountUsageStats(ctx, time.Now().In(appTimeLocation)); err != nil {
		log.Printf("refresh codex keeper usage stats failed: %v", err)
	}
}

func keeperUsageStatsNextDelay(now time.Time) time.Duration {
	next := time.Date(now.Year(), now.Month(), now.Day(), keeperUsageStatsRunHour, keeperUsageStatsRunMinute, 0, 0, appTimeLocation)
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}
	return next.Sub(now)
}

func (a *App) refreshKeeperAccountUsageStats(ctx context.Context, now time.Time) error {
	accounts, err := a.listKeeperAccounts(ctx)
	if err != nil {
		return err
	}
	if len(accounts) == 0 {
		_, err := a.db.ExecContext(ctx, `DELETE FROM codex_keeper_account_usage_stats`)
		return err
	}

	prices, err := a.priceMap(ctx)
	if err != nil {
		return err
	}
	accountByName := map[string]keeperAccount{}
	sourceAccounts := map[string]string{}
	aliases := map[string][]string{}
	for _, account := range accounts {
		accountByName[account.Name] = account
		addKeeperAuthAlias(aliases, account.Name, account.Name)
		addKeeperSourceAccountAlias(sourceAccounts, account.Name, account.Name)
		if account.Email != nil {
			addKeeperAuthAlias(aliases, *account.Email, account.Name)
			addKeeperSourceAccountAlias(sourceAccounts, *account.Email, account.Name)
		}
	}

	periods := map[string]*keeperUsageStatsPeriod{}
	rows, err := a.db.QueryContext(ctx, `SELECT `+usageRecordSelectColumns(true)+`
		FROM `+usageRecordsFrom(true)+`
		ORDER BY timestamp`)
	if err != nil {
		return err
	}
	records, err := scanUsageRecords(rows)
	if closeErr := rows.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}

	for _, record := range records {
		authName, ok := keeperAccountNameForUsageRecord(record, sourceAccounts, aliases)
		if !ok {
			continue
		}
		account, ok := accountByName[authName]
		if !ok {
			continue
		}
		day := keeperStatsStartOfDay(record.Timestamp)
		week := keeperStatsStartOfWeek(record.Timestamp)
		addKeeperUsageStatsRecord(keeperUsageStatsPeriodFor(periods, account, "day", day, day.AddDate(0, 0, 1), now), record, prices)
		addKeeperUsageStatsRecord(keeperUsageStatsPeriodFor(periods, account, "week", week, week.AddDate(0, 0, 7), now), record, prices)
	}

	today := keeperStatsStartOfDay(now)
	thisWeek := keeperStatsStartOfWeek(now)
	for _, account := range accounts {
		keeperUsageStatsPeriodFor(periods, account, "day", today, today.AddDate(0, 0, 1), now)
		keeperUsageStatsPeriodFor(periods, account, "week", thisWeek, thisWeek.AddDate(0, 0, 7), now)
	}

	return a.replaceKeeperUsageStats(ctx, periods)
}

func keeperUsageStatsPeriodFor(periods map[string]*keeperUsageStatsPeriod, account keeperAccount, periodType string, start, end, generatedAt time.Time) *keeperUsageStatsPeriod {
	key := account.Name + "|" + periodType + "|" + start.Format("2006-01-02")
	if period, ok := periods[key]; ok {
		return period
	}
	period := &keeperUsageStatsPeriod{
		AuthName:    account.Name,
		Email:       account.Email,
		AccountType: account.AccountType,
		PeriodType:  periodType,
		PeriodStart: start.In(appTimeLocation),
		PeriodEnd:   end.In(appTimeLocation),
		GeneratedAt: generatedAt.In(appTimeLocation),
	}
	periods[key] = period
	return period
}

func addKeeperUsageStatsRecord(period *keeperUsageStatsPeriod, record UsageRecord, prices map[[2]string]ModelPrice) {
	addRecordToKeeperUsageStatsMetric(&period.Metric, record, prices)
}

func addRecordToKeeperUsageStatsMetric(metric *keeperUsageStatsMetric, record UsageRecord, prices map[[2]string]ModelPrice) {
	metric.Records++
	if record.Failed {
		metric.FailedRecords++
	} else {
		metric.SuccessRecords++
	}
	metric.InputTokens += usageAggregateInputTokens(record)
	metric.OutputTokens += record.OutputTokens
	metric.CachedTokens += record.CachedTokens
	metric.CacheReadTokens += record.CacheReadTokens
	metric.CacheCreationTokens += record.CacheCreationTokens
	metric.ReasoningTokens += record.ReasoningTokens
	metric.TotalTokens += usageAggregateTotalTokens(record)
	amount, unpriced := recordCost(record, prices)
	if unpriced {
		metric.UnpricedRecords++
	} else {
		metric.EstimatedCostUSD = mathRound(metric.EstimatedCostUSD+amount, 8)
	}
	updateKeeperUsageStatsMetricBounds(metric, record.Timestamp)
}

func updateKeeperUsageStatsMetricBounds(metric *keeperUsageStatsMetric, timestamp time.Time) {
	value := timestamp.In(appTimeLocation)
	if metric.FirstRequestAt == nil || value.Before(*metric.FirstRequestAt) {
		copied := value
		metric.FirstRequestAt = &copied
	}
	if metric.LastRequestAt == nil || value.After(*metric.LastRequestAt) {
		copied := value
		metric.LastRequestAt = &copied
	}
}

func mergeKeeperUsageStatsMetric(target *keeperUsageStatsMetric, source keeperUsageStatsMetric) {
	target.Records += source.Records
	target.SuccessRecords += source.SuccessRecords
	target.FailedRecords += source.FailedRecords
	target.InputTokens += source.InputTokens
	target.OutputTokens += source.OutputTokens
	target.CachedTokens += source.CachedTokens
	target.CacheReadTokens += source.CacheReadTokens
	target.CacheCreationTokens += source.CacheCreationTokens
	target.ReasoningTokens += source.ReasoningTokens
	target.TotalTokens += source.TotalTokens
	target.EstimatedCostUSD = mathRound(target.EstimatedCostUSD+source.EstimatedCostUSD, 8)
	target.UnpricedRecords += source.UnpricedRecords
	if source.FirstRequestAt != nil {
		updateKeeperUsageStatsMetricBounds(target, *source.FirstRequestAt)
	}
	if source.LastRequestAt != nil {
		updateKeeperUsageStatsMetricBounds(target, *source.LastRequestAt)
	}
}

func (a *App) replaceKeeperUsageStats(ctx context.Context, periods map[string]*keeperUsageStatsPeriod) error {
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM codex_keeper_account_usage_stats`); err != nil {
		return err
	}
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO codex_keeper_account_usage_stats (
			auth_name, email, account_type, period_type, period_start, period_end,
			records, success_records, failed_records, input_tokens, output_tokens,
			cached_tokens, cache_read_tokens, cache_creation_tokens, reasoning_tokens,
			total_tokens, estimated_cost_usd, unpriced_records, first_request_at,
			last_request_at, generated_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	sorted := make([]*keeperUsageStatsPeriod, 0, len(periods))
	for _, period := range periods {
		sorted = append(sorted, period)
	}
	sort.Slice(sorted, func(i, j int) bool {
		left, right := sorted[i], sorted[j]
		if left.AuthName != right.AuthName {
			return left.AuthName < right.AuthName
		}
		if left.PeriodType != right.PeriodType {
			return left.PeriodType < right.PeriodType
		}
		return left.PeriodStart.Before(right.PeriodStart)
	})

	now := dbTime(time.Now().In(appTimeLocation))
	for _, period := range sorted {
		metric := period.Metric
		if _, err := stmt.ExecContext(
			ctx,
			period.AuthName,
			period.Email,
			period.AccountType,
			period.PeriodType,
			dbTime(period.PeriodStart),
			dbTime(period.PeriodEnd),
			metric.Records,
			metric.SuccessRecords,
			metric.FailedRecords,
			metric.InputTokens,
			metric.OutputTokens,
			metric.CachedTokens,
			metric.CacheReadTokens,
			metric.CacheCreationTokens,
			metric.ReasoningTokens,
			metric.TotalTokens,
			metric.EstimatedCostUSD,
			metric.UnpricedRecords,
			dbTimePtr(metric.FirstRequestAt),
			dbTimePtr(metric.LastRequestAt),
			dbTime(period.GeneratedAt),
			now,
			now,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (a *App) keeperUsageStatsSummaries(ctx context.Context, accounts []keeperAccount, now time.Time) (map[string]keeperUsageStatsSummaryResponse, error) {
	if err := a.ensureKeeperUsageStatsAvailable(ctx, accounts, now); err != nil {
		return nil, err
	}
	periods, err := a.loadKeeperUsageStatsPeriods(ctx, nil)
	if err != nil {
		return nil, err
	}
	createdAt, err := a.keeperAccountCreatedAtMap(ctx)
	if err != nil {
		return nil, err
	}
	accountNames := map[string]bool{}
	for _, account := range accounts {
		accountNames[account.Name] = true
	}
	return keeperUsageStatsSummariesFromPeriods(accounts, createdAt, periods, accountNames, now), nil
}

func (a *App) keeperAccountUsageStats(ctx context.Context, authName string, now time.Time) (keeperAccountUsageStatsResponse, error) {
	state, err := a.getKeeperState(ctx, authName)
	if err != nil {
		return keeperAccountUsageStatsResponse{}, err
	}
	account := state.keeperAccount
	if err := a.ensureKeeperUsageStatsAvailable(ctx, []keeperAccount{account}, now); err != nil {
		return keeperAccountUsageStatsResponse{}, err
	}
	periods, err := a.loadKeeperUsageStatsPeriods(ctx, &authName)
	if err != nil {
		return keeperAccountUsageStatsResponse{}, err
	}
	createdAt := map[string]time.Time{authName: state.CreatedAt}
	accountNames := map[string]bool{authName: true}
	summaries := keeperUsageStatsSummariesFromPeriods([]keeperAccount{account}, createdAt, periods, accountNames, now)
	response := keeperAccountUsageStatsResponse{
		AuthName:    authName,
		Email:       account.Email,
		AccountType: account.AccountType,
		Summary:     summaries[authName],
		Daily:       []keeperUsageStatsPeriodResponse{},
		Weekly:      []keeperUsageStatsPeriodResponse{},
	}
	for _, period := range periods {
		item := keeperUsageStatsPeriodResponseFrom(period)
		if period.PeriodType == "week" {
			response.Weekly = append(response.Weekly, item)
		} else {
			response.Daily = append(response.Daily, item)
		}
	}
	sort.Slice(response.Weekly, func(i, j int) bool {
		return response.Weekly[i].PeriodStart > response.Weekly[j].PeriodStart
	})
	sort.Slice(response.Daily, func(i, j int) bool {
		return response.Daily[i].PeriodStart > response.Daily[j].PeriodStart
	})
	return response, nil
}

func (a *App) ensureKeeperUsageStatsAvailable(ctx context.Context, accounts []keeperAccount, now time.Time) error {
	if len(accounts) == 0 {
		return nil
	}
	rows, err := a.db.QueryContext(ctx, `
		SELECT auth_name, CAST(MAX(generated_at) AS TEXT)
		FROM codex_keeper_account_usage_stats
		GROUP BY auth_name
	`)
	if err != nil {
		return err
	}
	defer rows.Close()
	seen := map[string]bool{}
	var latest time.Time
	for rows.Next() {
		var name string
		var generated sql.NullString
		if err := rows.Scan(&name, &generated); err != nil {
			return err
		}
		seen[name] = true
		if generated.Valid {
			if parsed, ok := parseDBTime(generated.String); ok && (latest.IsZero() || parsed.After(latest)) {
				latest = parsed
			}
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	needsRefresh := latest.IsZero() || latest.Before(now.In(appTimeLocation).Add(-25*time.Hour))
	if !needsRefresh {
		for _, account := range accounts {
			if !seen[account.Name] {
				needsRefresh = true
				break
			}
		}
	}
	if !needsRefresh {
		return nil
	}
	return a.refreshKeeperAccountUsageStats(ctx, now)
}

func (a *App) loadKeeperUsageStatsPeriods(ctx context.Context, authName *string) ([]keeperUsageStatsPeriod, error) {
	query := `
		SELECT auth_name, email, account_type, period_type, CAST(period_start AS TEXT),
		       CAST(period_end AS TEXT), records, success_records, failed_records,
		       input_tokens, output_tokens, cached_tokens, cache_read_tokens,
		       cache_creation_tokens, reasoning_tokens, total_tokens, estimated_cost_usd,
		       unpriced_records, CAST(first_request_at AS TEXT), CAST(last_request_at AS TEXT),
		       CAST(generated_at AS TEXT)
		FROM codex_keeper_account_usage_stats
	`
	args := []any{}
	if authName != nil {
		query += ` WHERE auth_name = ?`
		args = append(args, *authName)
	}
	query += ` ORDER BY auth_name, period_type, period_start`

	rows, err := a.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	periods := []keeperUsageStatsPeriod{}
	for rows.Next() {
		period, err := scanKeeperUsageStatsPeriod(rows)
		if err != nil {
			return nil, err
		}
		periods = append(periods, period)
	}
	return periods, rows.Err()
}

func scanKeeperUsageStatsPeriod(scanner interface{ Scan(dest ...any) error }) (keeperUsageStatsPeriod, error) {
	var period keeperUsageStatsPeriod
	var email, accountType, periodStart, periodEnd, firstRequestAt, lastRequestAt, generatedAt sql.NullString
	err := scanner.Scan(
		&period.AuthName,
		&email,
		&accountType,
		&period.PeriodType,
		&periodStart,
		&periodEnd,
		&period.Metric.Records,
		&period.Metric.SuccessRecords,
		&period.Metric.FailedRecords,
		&period.Metric.InputTokens,
		&period.Metric.OutputTokens,
		&period.Metric.CachedTokens,
		&period.Metric.CacheReadTokens,
		&period.Metric.CacheCreationTokens,
		&period.Metric.ReasoningTokens,
		&period.Metric.TotalTokens,
		&period.Metric.EstimatedCostUSD,
		&period.Metric.UnpricedRecords,
		&firstRequestAt,
		&lastRequestAt,
		&generatedAt,
	)
	if err != nil {
		return keeperUsageStatsPeriod{}, err
	}
	period.Email = nullableString(email)
	period.AccountType = nullableString(accountType)
	if parsed, ok := parseDBTime(periodStart.String); ok {
		period.PeriodStart = parsed
	}
	if parsed, ok := parseDBTime(periodEnd.String); ok {
		period.PeriodEnd = parsed
	}
	period.Metric.FirstRequestAt = timePtr(firstRequestAt)
	period.Metric.LastRequestAt = timePtr(lastRequestAt)
	if parsed, ok := parseDBTime(generatedAt.String); ok {
		period.GeneratedAt = parsed
	}
	return period, nil
}

func (a *App) keeperAccountCreatedAtMap(ctx context.Context) (map[string]time.Time, error) {
	rows, err := a.db.QueryContext(ctx, `SELECT auth_name, CAST(created_at AS TEXT) FROM codex_keeper_auth_states`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[string]time.Time{}
	for rows.Next() {
		var name string
		var created sql.NullString
		if err := rows.Scan(&name, &created); err != nil {
			return nil, err
		}
		if parsed, ok := parseDBTime(created.String); ok {
			result[name] = parsed
		}
	}
	return result, rows.Err()
}

func keeperUsageStatsSummariesFromPeriods(accounts []keeperAccount, createdAt map[string]time.Time, periods []keeperUsageStatsPeriod, accountNames map[string]bool, now time.Time) map[string]keeperUsageStatsSummaryResponse {
	today := keeperStatsStartOfDay(now)
	yesterday := today.AddDate(0, 0, -1)
	thisWeek := keeperStatsStartOfWeek(now)
	lastWeek := thisWeek.AddDate(0, 0, -7)
	twoWeeksAgo := thisWeek.AddDate(0, 0, -14)

	type accountPeriodMetrics struct {
		days        map[string]keeperUsageStatsMetric
		weeks       map[string]keeperUsageStatsMetric
		generatedAt *time.Time
	}
	grouped := map[string]*accountPeriodMetrics{}
	for _, account := range accounts {
		grouped[account.Name] = &accountPeriodMetrics{
			days:  map[string]keeperUsageStatsMetric{},
			weeks: map[string]keeperUsageStatsMetric{},
		}
	}
	for _, period := range periods {
		if !accountNames[period.AuthName] {
			continue
		}
		bucket := grouped[period.AuthName]
		if bucket == nil {
			bucket = &accountPeriodMetrics{
				days:  map[string]keeperUsageStatsMetric{},
				weeks: map[string]keeperUsageStatsMetric{},
			}
			grouped[period.AuthName] = bucket
		}
		key := keeperStatsPeriodKey(period.PeriodStart)
		if period.PeriodType == "week" {
			bucket.weeks[key] = period.Metric
		} else {
			bucket.days[key] = period.Metric
		}
		if bucket.generatedAt == nil || period.GeneratedAt.After(*bucket.generatedAt) {
			generated := period.GeneratedAt
			bucket.generatedAt = &generated
		}
	}

	summaries := map[string]keeperUsageStatsSummaryResponse{}
	for _, account := range accounts {
		bucket := grouped[account.Name]
		if bucket == nil {
			bucket = &accountPeriodMetrics{
				days:  map[string]keeperUsageStatsMetric{},
				weeks: map[string]keeperUsageStatsMetric{},
			}
		}
		allTime := keeperUsageStatsMetric{}
		activeDays := 0
		for _, metric := range bucket.days {
			if metric.Records > 0 {
				activeDays++
			}
			mergeKeeperUsageStatsMetric(&allTime, metric)
		}
		activeWeeks := 0
		for _, metric := range bucket.weeks {
			if metric.Records > 0 {
				activeWeeks++
			}
		}
		startAt := createdAt[account.Name]
		if startAt.IsZero() && allTime.FirstRequestAt != nil {
			startAt = *allTime.FirstRequestAt
		}
		aliveDays := keeperUsageStatsAliveDays(startAt, now)
		summary := keeperUsageStatsSummaryResponse{
			AliveDays:       aliveDays,
			AliveWeeks:      keeperUsageStatsAliveWeeks(aliveDays),
			ActiveDays:      activeDays,
			ActiveWeeks:     activeWeeks,
			FirstRequestAt:  apiDateTimePtr(allTime.FirstRequestAt),
			LastRequestAt:   apiDateTimePtr(allTime.LastRequestAt),
			LastGeneratedAt: apiDateTimePtr(bucket.generatedAt),
			Today:           keeperUsageStatsMetricResponseFrom(bucket.days[keeperStatsPeriodKey(today)]),
			Yesterday:       keeperUsageStatsMetricResponseFrom(bucket.days[keeperStatsPeriodKey(yesterday)]),
			ThisWeek:        keeperUsageStatsMetricResponseFrom(bucket.weeks[keeperStatsPeriodKey(thisWeek)]),
			LastWeek:        keeperUsageStatsMetricResponseFrom(bucket.weeks[keeperStatsPeriodKey(lastWeek)]),
			TwoWeeksAgo:     keeperUsageStatsMetricResponseFrom(bucket.weeks[keeperStatsPeriodKey(twoWeeksAgo)]),
			AllTime:         keeperUsageStatsMetricResponseFrom(allTime),
		}
		summaries[account.Name] = summary
	}
	return summaries
}

func keeperUsageStatsMetricResponseFrom(metric keeperUsageStatsMetric) keeperUsageStatsMetricResponse {
	return keeperUsageStatsMetricResponse{
		Records:             metric.Records,
		SuccessRecords:      metric.SuccessRecords,
		FailedRecords:       metric.FailedRecords,
		InputTokens:         metric.InputTokens,
		OutputTokens:        metric.OutputTokens,
		CachedTokens:        metric.CachedTokens,
		CacheReadTokens:     metric.CacheReadTokens,
		CacheCreationTokens: metric.CacheCreationTokens,
		ReasoningTokens:     metric.ReasoningTokens,
		TotalTokens:         metric.TotalTokens,
		EstimatedCostUSD:    metric.EstimatedCostUSD,
		UnpricedRecords:     metric.UnpricedRecords,
		FirstRequestAt:      apiDateTimePtr(metric.FirstRequestAt),
		LastRequestAt:       apiDateTimePtr(metric.LastRequestAt),
	}
}

func keeperUsageStatsPeriodResponseFrom(period keeperUsageStatsPeriod) keeperUsageStatsPeriodResponse {
	metric := keeperUsageStatsMetricResponseFrom(period.Metric)
	return keeperUsageStatsPeriodResponse{
		PeriodType:          period.PeriodType,
		PeriodStart:         apiDateTime(period.PeriodStart),
		PeriodEnd:           apiDateTime(period.PeriodEnd),
		Label:               keeperUsageStatsPeriodLabel(period),
		Records:             metric.Records,
		SuccessRecords:      metric.SuccessRecords,
		FailedRecords:       metric.FailedRecords,
		InputTokens:         metric.InputTokens,
		OutputTokens:        metric.OutputTokens,
		CachedTokens:        metric.CachedTokens,
		CacheReadTokens:     metric.CacheReadTokens,
		CacheCreationTokens: metric.CacheCreationTokens,
		ReasoningTokens:     metric.ReasoningTokens,
		TotalTokens:         metric.TotalTokens,
		EstimatedCostUSD:    metric.EstimatedCostUSD,
		UnpricedRecords:     metric.UnpricedRecords,
		FirstRequestAt:      metric.FirstRequestAt,
		LastRequestAt:       metric.LastRequestAt,
		GeneratedAt:         apiDateTime(period.GeneratedAt),
	}
}

func keeperUsageStatsPeriodLabel(period keeperUsageStatsPeriod) string {
	start := period.PeriodStart.In(appTimeLocation)
	if period.PeriodType == "week" {
		end := period.PeriodEnd.In(appTimeLocation).AddDate(0, 0, -1)
		return start.Format("2006-01-02") + " 至 " + end.Format("2006-01-02")
	}
	return start.Format("2006-01-02")
}

func keeperStatsStartOfDay(value time.Time) time.Time {
	local := value.In(appTimeLocation)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, appTimeLocation)
}

func keeperStatsStartOfWeek(value time.Time) time.Time {
	day := keeperStatsStartOfDay(value)
	weekday := int(day.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	return day.AddDate(0, 0, -(weekday - 1))
}

func keeperStatsPeriodKey(value time.Time) string {
	return value.In(appTimeLocation).Format("2006-01-02")
}

func keeperUsageStatsAliveDays(startAt time.Time, now time.Time) int {
	if startAt.IsZero() {
		return 0
	}
	start := keeperStatsStartOfDay(startAt)
	end := keeperStatsStartOfDay(now)
	days := int(end.Sub(start).Hours()/24) + 1
	if days < 0 {
		return 0
	}
	return days
}

func keeperUsageStatsAliveWeeks(aliveDays int) int {
	if aliveDays <= 0 {
		return 0
	}
	return (aliveDays + 6) / 7
}

func normalizeKeeperStatsAuthName(value string) (string, error) {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return "", validationError("账号名称无效")
	}
	if strings.Contains(normalized, "/") || strings.Contains(normalized, "\\") {
		return "", validationError("账号名称无效")
	}
	return normalized, nil
}
