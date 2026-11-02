package services

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"

	"rudy/internal/models"
	"rudy/internal/transaction"
)

type StatsService struct {
	db *transaction.DB
}

func NewStatsService(db *transaction.DB) *StatsService {
	return &StatsService{db: db}
}

func (s *StatsService) Stats(ctx context.Context, requestedRange string) (models.Stats, error) {
	now := nowUTC()
	rangeKey, start, bounded := normalizeRange(requestedRange, now)

	since := ""
	if bounded {
		since = start.Format(transaction.TimeLayout())
	}

	stats := models.Stats{
		Range:                rangeKey,
		TasksByState:         map[string]int{"pending": 0, "auditing": 0, "completed": 0},
		TasksByApprovalState: map[string]int{"proposed": 0, "approved": 0, "changes_requested": 0},
		TasksByType:          map[string]int{"feature": 0, "bug": 0, "fix": 0, "research": 0},
		RoleDistribution:     map[string]int{},
		Projects:             []models.ProjectStats{},
		Users:                []models.UserStats{},
		Trends:               []models.TrendPoint{},
		LedgerByReason:       []models.ReasonTotal{},
	}

	var err error
	if stats.Totals.Users, err = s.count(ctx, `SELECT COUNT(*) FROM users`, bounded, since, "created_at"); err != nil {
		return models.Stats{}, err
	}
	if stats.Totals.Builders, err = s.count(ctx, `SELECT COUNT(*) FROM users WHERE role <> 'Lead'`, bounded, since, "created_at"); err != nil {
		return models.Stats{}, err
	}
	if stats.Totals.Projects, err = s.count(ctx, `SELECT COUNT(*) FROM projects`, bounded, since, "created_at"); err != nil {
		return models.Stats{}, err
	}
	if stats.Totals.Tasks, err = s.count(ctx, `SELECT COUNT(*) FROM tasks`, bounded, since, "created_at"); err != nil {
		return models.Stats{}, err
	}
	if stats.Totals.CompletedTasks, err = s.count(ctx,
		`SELECT COUNT(*) FROM tasks WHERE task_state = 'completed'`, bounded, since, "completed_at"); err != nil {
		return models.Stats{}, err
	}
	if stats.Totals.Tasks > 0 {
		stats.Totals.CompletionRate = round2(float64(stats.Totals.CompletedTasks) / float64(stats.Totals.Tasks) * 100)
	}

	pointsQuery := s.filteredQuery(
		`SELECT COALESCE(SUM(points), 0) FROM ledger_entries WHERE reason IN ('task_completed', 'review_bonus')`,
		bounded, since, "created_at",
	)
	pointsArgs := []any{}
	if bounded {
		pointsArgs = append(pointsArgs, since)
	}
	if err := s.db.SQL().QueryRowContext(ctx, pointsQuery, pointsArgs...).Scan(&stats.Totals.PointsMinted); err != nil {
		return models.Stats{}, err
	}

	if stats.Totals.ReviewsApproved, err = s.count(ctx,
		`SELECT COUNT(*) FROM task_reviewers WHERE decision = 'approved'`, bounded, since, "decided_at"); err != nil {
		return models.Stats{}, err
	}
	if stats.Totals.ReviewsDecided, err = s.count(ctx,
		`SELECT COUNT(*) FROM task_reviewers WHERE decision <> 'pending'`, bounded, since, "decided_at"); err != nil {
		return models.Stats{}, err
	}
	if stats.Totals.ReviewsDecided > 0 {
		stats.Totals.ReviewApprovalPct = round2(float64(stats.Totals.ReviewsApproved) / float64(stats.Totals.ReviewsDecided) * 100)
	}

	var avgCycle sql.NullFloat64
	query := s.filteredQuery(`
		SELECT AVG((julianday(completed_at) - julianday(created_at)) * 24)
		FROM tasks WHERE completed_at IS NOT NULL`, bounded, since, "completed_at")
	avgArgs := []any{}
	if bounded {
		avgArgs = append(avgArgs, since)
	}
	if err := s.db.SQL().QueryRowContext(ctx, query, avgArgs...).Scan(&avgCycle); err != nil {
		return models.Stats{}, err
	}
	if avgCycle.Valid {
		stats.Totals.AvgCycleHours = round2(avgCycle.Float64)
	}

	if stats.TasksByState, err = s.groupCounts(ctx, `SELECT task_state, COUNT(*) FROM tasks`, bounded, since, "created_at"); err != nil {
		return models.Stats{}, err
	}
	if stats.TasksByApprovalState, err = s.groupCounts(ctx, `SELECT approval_state, COUNT(*) FROM tasks`, bounded, since, "created_at"); err != nil {
		return models.Stats{}, err
	}
	if stats.TasksByType, err = s.groupCounts(ctx, `SELECT task_type, COUNT(*) FROM tasks`, bounded, since, "created_at"); err != nil {
		return models.Stats{}, err
	}
	if stats.RoleDistribution, err = s.groupCounts(ctx, `SELECT role, COUNT(*) FROM users`, bounded, since, "created_at"); err != nil {
		return models.Stats{}, err
	}

	if stats.Projects, err = s.projectStats(ctx, since); err != nil {
		return models.Stats{}, err
	}
	if stats.Users, err = s.userStats(ctx); err != nil {
		return models.Stats{}, err
	}
	if stats.Trends, err = s.trends(ctx, start, now, bounded); err != nil {
		return models.Stats{}, err
	}
	if stats.LedgerByReason, err = s.ledgerByReason(ctx, bounded, since); err != nil {
		return models.Stats{}, err
	}

	return stats, nil
}

func normalizeRange(value string, now time.Time) (string, time.Time, bool) {
	switch value {
	case "all":
		return "all", time.Time{}, false
	case "90d":
		return "90d", now.AddDate(0, 0, -90), true
	case "7d":
		return "7d", now.AddDate(0, 0, -7), true
	case "", "30d":
		return "30d", now.AddDate(0, 0, -30), true
	default:
		return "30d", now.AddDate(0, 0, -30), true
	}
}

func (s *StatsService) filteredQuery(base string, bounded bool, since, column string) string {
	if !bounded {
		return base
	}
	if containsWhere(base) {
		return fmt.Sprintf(`%s AND %s >= ?`, base, column)
	}
	return fmt.Sprintf(`%s WHERE %s >= ?`, base, column)
}

func containsWhere(base string) bool {
	for index := 0; index+5 <= len(base); index++ {
		if base[index:index+5] == "WHERE" {
			return true
		}
	}
	return false
}

func (s *StatsService) count(ctx context.Context, base string, bounded bool, since, column string) (int, error) {
	var value int
	query := s.filteredQuery(base, bounded, since, column)
	args := []any{}
	if bounded {
		args = append(args, since)
	}
	err := s.db.SQL().QueryRowContext(ctx, query, args...).Scan(&value)
	return value, err
}

func (s *StatsService) groupCounts(ctx context.Context, base string, bounded bool, since, column string) (map[string]int, error) {
	query := s.filteredQuery(base, bounded, since, column) + ` GROUP BY 1`
	args := []any{}
	if bounded {
		args = append(args, since)
	}
	rows, err := s.db.SQL().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := map[string]int{}
	for rows.Next() {
		var (
			key   string
			count int
		)
		if err := rows.Scan(&key, &count); err != nil {
			return nil, err
		}
		result[key] = count
	}
	return result, rows.Err()
}

func (s *StatsService) projectStats(ctx context.Context, since string) ([]models.ProjectStats, error) {
	rows, err := s.db.SQL().QueryContext(ctx, `
		SELECT
			p.id, p.name,
			COUNT(t.id) AS total,
			COALESCE(SUM(CASE WHEN t.approval_state = 'proposed' OR t.approval_state = 'changes_requested' THEN 1 ELSE 0 END), 0) AS proposed,
			COALESCE(SUM(CASE WHEN t.task_state = 'pending' THEN 1 ELSE 0 END), 0) AS pending,
			COALESCE(SUM(CASE WHEN t.task_state = 'auditing' THEN 1 ELSE 0 END), 0) AS auditing,
			COALESCE(SUM(CASE WHEN t.task_state = 'completed' THEN 1 ELSE 0 END), 0) AS completed,
			COALESCE(SUM(CASE WHEN t.task_state <> 'completed' THEN t.points ELSE 0 END), 0) AS open_points,
			COALESCE(SUM(CASE WHEN t.task_state = 'completed' THEN t.points ELSE 0 END), 0) AS paid_points
		FROM projects p
		LEFT JOIN tasks t ON t.project_id = p.id AND (? = '' OR t.created_at >= ?)
		GROUP BY p.id, p.name
		ORDER BY p.created_at DESC, p.id DESC`, since, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []models.ProjectStats{}
	for rows.Next() {
		var item models.ProjectStats
		if err := rows.Scan(
			&item.ProjectID, &item.Name, &item.TotalTasks, &item.Proposed,
			&item.Pending, &item.Auditing, &item.Completed, &item.OpenPoints, &item.PaidPoints,
		); err != nil {
			return nil, err
		}
		if item.TotalTasks > 0 {
			item.CompletionPct = round2(float64(item.Completed) / float64(item.TotalTasks) * 100)
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *StatsService) userStats(ctx context.Context) ([]models.UserStats, error) {
	rows, err := s.db.SQL().QueryContext(ctx, `
		SELECT
			u.id, u.username, u.role, u.points, u.created_at,
			(SELECT COUNT(*) FROM tasks t WHERE t.assignee_id = u.id AND t.task_state = 'completed') AS completed_tasks,
			(SELECT COUNT(*) FROM task_reviewers r WHERE r.user_id = u.id AND r.decision = 'approved') AS reviews
		FROM users u
		ORDER BY u.points DESC, completed_tasks DESC, u.username ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []models.UserStats{}
	for rows.Next() {
		var (
			item      models.UserStats
			createdAt string
		)
		if err := rows.Scan(
			&item.User.ID, &item.User.Username, &item.User.Role, &item.User.Points, &createdAt,
			&item.CompletedTasks, &item.Reviews,
		); err != nil {
			return nil, err
		}
		item.User.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *StatsService) trends(ctx context.Context, start, now time.Time, bounded bool) ([]models.TrendPoint, error) {
	points := map[string]*models.TrendPoint{}
	get := func(period string) *models.TrendPoint {
		if existing, ok := points[period]; ok {
			return existing
		}
		item := &models.TrendPoint{Period: period}
		points[period] = item
		return item
	}

	type series struct {
		query string
		apply func(*models.TrendPoint, int)
	}
	seriesList := []series{
		{
			query: `SELECT date(created_at), COUNT(*) FROM tasks`,
			apply: func(point *models.TrendPoint, value int) { point.Proposals = value },
		},
		{
			query: `SELECT date(completed_at), COUNT(*) FROM tasks WHERE completed_at IS NOT NULL`,
			apply: func(point *models.TrendPoint, value int) { point.Completions = value },
		},
		{
			query: `SELECT date(decided_at), COUNT(*) FROM task_reviewers WHERE decision = 'approved' AND decided_at IS NOT NULL`,
			apply: func(point *models.TrendPoint, value int) { point.Reviews = value },
		},
		{
			query: `SELECT date(created_at), COALESCE(SUM(points), 0) FROM ledger_entries WHERE reason IN ('task_completed', 'review_bonus')`,
			apply: func(point *models.TrendPoint, value int) { point.PointsMinted = value },
		},
	}

	for _, item := range seriesList {
		query := item.query
		args := []any{}
		if bounded {
			column := "created_at"
			if containsCompleted(item.query) {
				column = "completed_at"
			}
			if containsDecided(item.query) {
				column = "decided_at"
			}
			query = s.filteredQuery(query, true, start.Format(transaction.TimeLayout()), column)
			args = append(args, start.Format(transaction.TimeLayout()))
		}
		query += ` GROUP BY 1 ORDER BY 1`

		rows, err := s.db.SQL().QueryContext(ctx, query, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var (
				period string
				value  int
			)
			if err := rows.Scan(&period, &value); err != nil {
				rows.Close()
				return nil, err
			}
			item.apply(get(period), value)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
	}

	if bounded {
		for day := start; !day.After(now); day = day.AddDate(0, 0, 1) {
			get(day.Format("2006-01-02"))
		}
	}

	keys := make([]string, 0, len(points))
	for key := range points {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	result := make([]models.TrendPoint, 0, len(keys))
	for _, key := range keys {
		if key == "" || key == "0000-00-00" {
			continue
		}
		result = append(result, *points[key])
	}
	return result, nil
}

func containsCompleted(query string) bool {
	return len(query) >= 12 && containsSubstring(query, "completed_at")
}

func containsDecided(query string) bool {
	return containsSubstring(query, "decided_at")
}

func containsSubstring(haystack, needle string) bool {
	for index := 0; index+len(needle) <= len(haystack); index++ {
		if haystack[index:index+len(needle)] == needle {
			return true
		}
	}
	return false
}

func (s *StatsService) ledgerByReason(ctx context.Context, bounded bool, since string) ([]models.ReasonTotal, error) {
	query := s.filteredQuery(`SELECT reason, COALESCE(SUM(points), 0), COUNT(*) FROM ledger_entries`, bounded, since, "created_at") + ` GROUP BY reason ORDER BY reason`
	args := []any{}
	if bounded {
		args = append(args, since)
	}
	rows, err := s.db.SQL().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []models.ReasonTotal{}
	for rows.Next() {
		var item models.ReasonTotal
		if err := rows.Scan(&item.Reason, &item.Points, &item.Count); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func round2(value float64) float64 {
	return float64(int(value*100+0.5)) / 100
}
