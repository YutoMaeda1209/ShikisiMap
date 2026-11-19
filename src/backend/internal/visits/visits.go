// Package visits manages the visit records of the signed-in user.
package visits

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/auth"
	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/httpx"
)

const (
	// MaxBatchSize is the most visits one request can add.
	MaxBatchSize = 1000
	// MaxMemoLength is the limit of a memo, in characters.
	MaxMemoLength = 1000

	// CodeUnknownSpot: a visit refers to a spot that does not exist.
	CodeUnknownSpot = "unknown_spot"
	// CodeDuplicateVisit: an update would make two visits identical.
	CodeDuplicateVisit = "duplicate_visit"

	uniqueViolation = "23505"
)

// Visit is one visit of the signed-in user to a spot.
type Visit struct {
	ID        string    `json:"id"`
	SpotID    string    `json:"spotId"`
	VisitedAt time.Time `json:"visitedAt"`
	Memo      *string   `json:"memo"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

const visitColumns = `id, spot_id, visited_at, memo, created_at, updated_at`

func scanVisit(row pgx.CollectableRow) (Visit, error) {
	var v Visit
	err := row.Scan(&v.ID, &v.SpotID, &v.VisitedAt, &v.Memo, &v.CreatedAt, &v.UpdatedAt)
	v.VisitedAt, v.CreatedAt, v.UpdatedAt = v.VisitedAt.UTC(), v.CreatedAt.UTC(), v.UpdatedAt.UTC()
	return v, err
}

type handler struct {
	pool *pgxpool.Pool
}

// Register adds the endpoints to the API.
func Register(api gin.IRoutes, pool *pgxpool.Pool, authSvc *auth.Service) {
	h := &handler{pool: pool}
	api.GET("/me/visits", authSvc.RequireUser(), h.list)
	api.POST("/me/visits", authSvc.RequireUser(), h.create)
	api.PATCH("/me/visits/:id", authSvc.RequireUser(), h.update)
	api.DELETE("/me/visits/:id", authSvc.RequireUser(), h.delete)
}

// list serves the visits as JSON or, for the export, as CSV.
func (h *handler) list(c *gin.Context) {
	rows, err := h.pool.Query(c.Request.Context(), `
		SELECT v.id, v.spot_id, v.visited_at, v.memo, v.created_at, v.updated_at, s.name
		FROM visits v JOIN spots s ON s.id = v.spot_id
		WHERE v.user_id = $1
		ORDER BY v.visited_at DESC, v.id`, auth.UserID(c))
	if err != nil {
		httpx.InternalError(c, err)
		return
	}
	defer rows.Close()

	visits := []Visit{}
	var spotNames []string
	for rows.Next() {
		var v Visit
		var spotName string
		if err := rows.Scan(&v.ID, &v.SpotID, &v.VisitedAt, &v.Memo, &v.CreatedAt, &v.UpdatedAt, &spotName); err != nil {
			httpx.InternalError(c, err)
			return
		}
		v.VisitedAt, v.CreatedAt, v.UpdatedAt = v.VisitedAt.UTC(), v.CreatedAt.UTC(), v.UpdatedAt.UTC()
		visits = append(visits, v)
		spotNames = append(spotNames, spotName)
	}
	if err := rows.Err(); err != nil {
		httpx.InternalError(c, err)
		return
	}

	// The response depends on Accept, so caches must not mix the two up.
	c.Header("Vary", "Accept")
	if wantsCSV(c.GetHeader("Accept")) {
		writeCSV(c, visits, spotNames)
		return
	}
	c.JSON(http.StatusOK, gin.H{"visits": visits})
}

// wantsCSV reports whether the Accept header asks for CSV rather than JSON.
// JSON is the default, also for "*/*" and for a header naming both.
func wantsCSV(accept string) bool {
	csvAccepted := false
	for _, part := range strings.Split(accept, ",") {
		mediaType, _, _ := strings.Cut(part, ";")
		switch strings.ToLower(strings.TrimSpace(mediaType)) {
		case "application/json":
			return false
		case "text/csv":
			csvAccepted = true
		}
	}
	return csvAccepted
}

func writeCSV(c *gin.Context, visits []Visit, spotNames []string) {
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="shikisimap-visits.csv"`)
	c.Status(http.StatusOK)

	// The byte order mark makes Excel read the file as UTF-8.
	c.Writer.WriteString("\xEF\xBB\xBF")
	w := csv.NewWriter(c.Writer)
	w.Write([]string{"id", "spot_id", "spot_name", "visited_at", "memo"})
	for i, v := range visits {
		memo := ""
		if v.Memo != nil {
			memo = *v.Memo
		}
		w.Write([]string{v.ID, v.SpotID, csvText(spotNames[i]), v.VisitedAt.Format(time.RFC3339), csvText(memo)})
	}
	w.Flush()
}

// csvText keeps spreadsheet applications from running a cell as a formula.
func csvText(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}

// visitInput is a visit in a request. A nil VisitedAt or Memo was not given.
type visitInput struct {
	SpotID    string          `json:"spotId"`
	VisitedAt *time.Time      `json:"visitedAt"`
	Memo      json.RawMessage `json:"memo"`
}

// memoValue reads a memo field: given tells whether it was present, and the
// value is nil for null or an empty text.
func memoValue(raw json.RawMessage) (value *string, given bool, err error) {
	if raw == nil {
		return nil, false, nil
	}
	if string(raw) == "null" {
		return nil, true, nil
	}
	var memo string
	if err := json.Unmarshal(raw, &memo); err != nil {
		return nil, true, errors.New("memo must be a string or null")
	}
	if utf8.RuneCountInString(memo) > MaxMemoLength {
		return nil, true, fmt.Errorf("memo must have at most %d characters", MaxMemoLength)
	}
	if strings.TrimSpace(memo) == "" {
		return nil, true, nil
	}
	return &memo, true, nil
}

// checkVisitedAt rejects times PostgreSQL cannot store or that make no sense.
func checkVisitedAt(t time.Time) error {
	if t.Year() < 1900 || t.After(time.Now().Add(24*time.Hour)) {
		return errors.New("visitedAt must not be in the future or before 1900")
	}
	return nil
}

// create adds one or more visits in a single transaction.
func (h *handler) create(c *gin.Context) {
	var inputs []visitInput
	if !httpx.BindJSON(c, &inputs) {
		return
	}
	if len(inputs) == 0 || len(inputs) > MaxBatchSize {
		httpx.ValidationError(c, fmt.Sprintf("the body must be an array of 1 to %d visits", MaxBatchSize))
		return
	}

	now := time.Now()
	type row struct {
		spotID    string
		visitedAt time.Time
		memo      *string
	}
	rows := make([]row, len(inputs))
	spotIDs := make([]string, len(inputs))
	for i, in := range inputs {
		if !httpx.IsUUID(in.SpotID) {
			httpx.ValidationError(c, fmt.Sprintf("visits[%d].spotId must be a UUID", i))
			return
		}
		visitedAt := now
		if in.VisitedAt != nil {
			visitedAt = *in.VisitedAt
		}
		if err := checkVisitedAt(visitedAt); err != nil {
			httpx.ValidationError(c, fmt.Sprintf("visits[%d]: %v", i, err))
			return
		}
		memo, _, err := memoValue(in.Memo)
		if err != nil {
			httpx.ValidationError(c, fmt.Sprintf("visits[%d]: %v", i, err))
			return
		}
		// PostgreSQL keeps microseconds; truncating first makes the
		// duplicate check see what will be stored.
		rows[i] = row{strings.ToLower(in.SpotID), visitedAt.Truncate(time.Microsecond), memo}
		spotIDs[i] = rows[i].spotID
	}

	ctx := c.Request.Context()
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		httpx.InternalError(c, err)
		return
	}
	defer tx.Rollback(ctx)

	var unknown []string
	err = tx.QueryRow(ctx, `
		SELECT COALESCE(array_agg(DISTINCT wanted), '{}') FROM unnest($1::uuid[]) AS wanted
		WHERE NOT EXISTS (SELECT 1 FROM spots s WHERE s.id = wanted)`, spotIDs).Scan(&unknown)
	if err != nil {
		httpx.InternalError(c, err)
		return
	}
	if len(unknown) > 0 {
		httpx.Error(c, http.StatusUnprocessableEntity, CodeUnknownSpot,
			"no visit was added because these spots do not exist: "+strings.Join(unknown, ", "))
		return
	}

	created := []Visit{}
	for _, r := range rows {
		result, err := tx.Query(ctx, `
			INSERT INTO visits (user_id, spot_id, visited_at, memo) VALUES ($1, $2, $3, $4)
			ON CONFLICT (user_id, spot_id, visited_at) DO NOTHING
			RETURNING `+visitColumns, auth.UserID(c), r.spotID, r.visitedAt, r.memo)
		if err != nil {
			httpx.InternalError(c, err)
			return
		}
		inserted, err := pgx.CollectRows(result, scanVisit)
		if err != nil {
			httpx.InternalError(c, err)
			return
		}
		created = append(created, inserted...)
	}
	if err := tx.Commit(ctx); err != nil {
		httpx.InternalError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"visits": created, "skipped": len(rows) - len(created)})
}

func (h *handler) update(c *gin.Context) {
	const notFound = "no such visit"
	id := c.Param("id")
	var in struct {
		VisitedAt *time.Time      `json:"visitedAt"`
		Memo      json.RawMessage `json:"memo"`
	}
	if !httpx.BindJSON(c, &in) {
		return
	}
	if !httpx.IsUUID(id) {
		httpx.NotFound(c, notFound)
		return
	}
	if in.VisitedAt != nil {
		if err := checkVisitedAt(*in.VisitedAt); err != nil {
			httpx.ValidationError(c, err.Error())
			return
		}
	}
	memo, memoGiven, err := memoValue(in.Memo)
	if err != nil {
		httpx.ValidationError(c, err.Error())
		return
	}

	// user_id in the condition keeps users from touching each other's visits.
	rows, err := h.pool.Query(c.Request.Context(), `
		UPDATE visits SET
			visited_at = COALESCE($3, visited_at),
			memo = CASE WHEN $4 THEN $5 ELSE memo END
		WHERE id = $1 AND user_id = $2
		RETURNING `+visitColumns, id, auth.UserID(c), in.VisitedAt, memoGiven, memo)
	if err != nil {
		httpx.InternalError(c, err)
		return
	}
	updated, err := pgx.CollectRows(rows, scanVisit)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
		httpx.Error(c, http.StatusConflict, CodeDuplicateVisit, "another visit to the spot has the same date and time")
		return
	}
	if err != nil {
		httpx.InternalError(c, err)
		return
	}
	if len(updated) == 0 {
		httpx.NotFound(c, notFound)
		return
	}
	c.JSON(http.StatusOK, updated[0])
}

func (h *handler) delete(c *gin.Context) {
	const notFound = "no such visit"
	id := c.Param("id")
	if !httpx.IsUUID(id) {
		httpx.NotFound(c, notFound)
		return
	}
	tag, err := h.pool.Exec(c.Request.Context(),
		`DELETE FROM visits WHERE id = $1 AND user_id = $2`, id, auth.UserID(c))
	if err != nil {
		httpx.InternalError(c, err)
		return
	}
	if tag.RowsAffected() == 0 {
		httpx.NotFound(c, notFound)
		return
	}
	c.Status(http.StatusNoContent)
}
