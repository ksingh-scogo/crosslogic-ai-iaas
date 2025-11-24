package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/crosslogic/control-plane/pkg/models"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

// ModelResponse represents the API response for a model
type ModelResponse struct {
	ID                      string                 `json:"id"`
	Name                    string                 `json:"name"`
	Family                  string                 `json:"family"`
	Size                    *string                `json:"size,omitempty"`
	Type                    string                 `json:"type"`
	ContextLength           int                    `json:"context_length"`
	VRAMRequiredGB          int                    `json:"vram_required_gb"`
	PriceInputPerMillion    float64                `json:"price_input_per_million"`
	PriceOutputPerMillion   float64                `json:"price_output_per_million"`
	TokensPerSecondCapacity *int                   `json:"tokens_per_second_capacity,omitempty"`
	Status                  string                 `json:"status"`
	Metadata                map[string]interface{} `json:"metadata"`
	CreatedAt               time.Time              `json:"created_at"`
	UpdatedAt               time.Time              `json:"updated_at"`
}

// ModelCreateRequest represents the request body for creating a model
type ModelCreateRequest struct {
	Name                    string                 `json:"name"`
	Family                  string                 `json:"family"`
	Size                    *string                `json:"size,omitempty"`
	Type                    string                 `json:"type"`
	ContextLength           int                    `json:"context_length"`
	VRAMRequiredGB          int                    `json:"vram_required_gb"`
	PriceInputPerMillion    float64                `json:"price_input_per_million"`
	PriceOutputPerMillion   float64                `json:"price_output_per_million"`
	TokensPerSecondCapacity *int                   `json:"tokens_per_second_capacity,omitempty"`
	Status                  string                 `json:"status,omitempty"`
	Metadata                map[string]interface{} `json:"metadata,omitempty"`
}

// ModelUpdateRequest represents the request body for full update
type ModelUpdateRequest struct {
	Name                    string                 `json:"name"`
	Family                  string                 `json:"family"`
	Size                    *string                `json:"size,omitempty"`
	Type                    string                 `json:"type"`
	ContextLength           int                    `json:"context_length"`
	VRAMRequiredGB          int                    `json:"vram_required_gb"`
	PriceInputPerMillion    float64                `json:"price_input_per_million"`
	PriceOutputPerMillion   float64                `json:"price_output_per_million"`
	TokensPerSecondCapacity *int                   `json:"tokens_per_second_capacity,omitempty"`
	Status                  string                 `json:"status"`
	Metadata                map[string]interface{} `json:"metadata,omitempty"`
}

// ModelPatchRequest represents the request body for partial update
type ModelPatchRequest struct {
	Name                    *string                 `json:"name,omitempty"`
	Family                  *string                 `json:"family,omitempty"`
	Size                    *string                 `json:"size,omitempty"`
	Type                    *string                 `json:"type,omitempty"`
	ContextLength           *int                    `json:"context_length,omitempty"`
	VRAMRequiredGB          *int                    `json:"vram_required_gb,omitempty"`
	PriceInputPerMillion    *float64                `json:"price_input_per_million,omitempty"`
	PriceOutputPerMillion   *float64                `json:"price_output_per_million,omitempty"`
	TokensPerSecondCapacity *int                    `json:"tokens_per_second_capacity,omitempty"`
	Status                  *string                 `json:"status,omitempty"`
	Metadata                *map[string]interface{} `json:"metadata,omitempty"`
}

// PaginationResponse represents pagination metadata
type PaginationResponse struct {
	Total   int  `json:"total"`
	Limit   int  `json:"limit"`
	Offset  int  `json:"offset"`
	HasMore bool `json:"has_more"`
}

// HandleListModels handles GET /api/v1/admin/models
func (g *Gateway) HandleListModels(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query parameters
	limit := parseIntParam(r, "limit", 50, 1, 100)
	offset := parseIntParam(r, "offset", 0, 0, 999999)
	family := r.URL.Query().Get("family")
	modelType := r.URL.Query().Get("type")
	status := r.URL.Query().Get("status")
	minVRAM := parseIntParam(r, "min_vram", 0, 0, 99999)
	maxVRAM := parseIntParam(r, "max_vram", 0, 0, 99999)
	search := r.URL.Query().Get("search")
	sortBy := r.URL.Query().Get("sort_by")
	sortOrder := r.URL.Query().Get("sort_order")

	// Default sort
	if sortBy == "" {
		sortBy = "name"
	}
	if sortOrder == "" {
		sortOrder = "asc"
	}

	// Validate sort fields
	validSortFields := map[string]bool{
		"name":             true,
		"family":           true,
		"created_at":       true,
		"updated_at":       true,
		"vram_required_gb": true,
	}
	if !validSortFields[sortBy] {
		g.writeError(w, http.StatusBadRequest, "invalid sort_by field")
		return
	}

	if sortOrder != "asc" && sortOrder != "desc" {
		g.writeError(w, http.StatusBadRequest, "sort_order must be 'asc' or 'desc'")
		return
	}

	// Build query
	queryBuilder := strings.Builder{}
	queryBuilder.WriteString(`
		SELECT id, name, family, size, type, context_length,
		       vram_required_gb, price_input_per_million, price_output_per_million,
		       tokens_per_second_capacity, status, metadata, created_at, updated_at
		FROM models
		WHERE 1=1
	`)

	countBuilder := strings.Builder{}
	countBuilder.WriteString("SELECT COUNT(*) FROM models WHERE 1=1")

	args := []interface{}{}
	argIndex := 1

	// Add filters
	if family != "" {
		queryBuilder.WriteString(fmt.Sprintf(" AND family = $%d", argIndex))
		countBuilder.WriteString(fmt.Sprintf(" AND family = $%d", argIndex))
		args = append(args, family)
		argIndex++
	}

	if modelType != "" {
		queryBuilder.WriteString(fmt.Sprintf(" AND type = $%d", argIndex))
		countBuilder.WriteString(fmt.Sprintf(" AND type = $%d", argIndex))
		args = append(args, modelType)
		argIndex++
	}

	if status != "" {
		queryBuilder.WriteString(fmt.Sprintf(" AND status = $%d", argIndex))
		countBuilder.WriteString(fmt.Sprintf(" AND status = $%d", argIndex))
		args = append(args, status)
		argIndex++
	}

	if minVRAM > 0 {
		queryBuilder.WriteString(fmt.Sprintf(" AND vram_required_gb >= $%d", argIndex))
		countBuilder.WriteString(fmt.Sprintf(" AND vram_required_gb >= $%d", argIndex))
		args = append(args, minVRAM)
		argIndex++
	}

	if maxVRAM > 0 {
		queryBuilder.WriteString(fmt.Sprintf(" AND vram_required_gb <= $%d", argIndex))
		countBuilder.WriteString(fmt.Sprintf(" AND vram_required_gb <= $%d", argIndex))
		args = append(args, maxVRAM)
		argIndex++
	}

	if search != "" {
		queryBuilder.WriteString(fmt.Sprintf(" AND name ILIKE $%d", argIndex))
		countBuilder.WriteString(fmt.Sprintf(" AND name ILIKE $%d", argIndex))
		args = append(args, "%"+search+"%")
		argIndex++
	}

	// Add sorting
	queryBuilder.WriteString(fmt.Sprintf(" ORDER BY %s %s", sortBy, strings.ToUpper(sortOrder)))

	// Add pagination
	queryBuilder.WriteString(fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIndex, argIndex+1))
	args = append(args, limit, offset)

	// Get total count
	var total int
	err := g.db.Pool.QueryRow(ctx, countBuilder.String(), args[:len(args)-2]...).Scan(&total)
	if err != nil {
		g.logger.Error("failed to count models", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to count models")
		return
	}

	// Execute query
	rows, err := g.db.Pool.Query(ctx, queryBuilder.String(), args...)
	if err != nil {
		g.logger.Error("failed to query models", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to query models")
		return
	}
	defer rows.Close()

	modelsList := []ModelResponse{}
	for rows.Next() {
		var m models.Model
		var metadataJSON []byte

		err := rows.Scan(
			&m.ID, &m.Name, &m.Family, &m.Size, &m.Type, &m.ContextLength,
			&m.VRAMRequiredGB, &m.PriceInputPerMillion, &m.PriceOutputPerMillion,
			&m.TokensPerSecondCapacity, &m.Status, &metadataJSON, &m.CreatedAt, &m.UpdatedAt,
		)
		if err != nil {
			g.logger.Error("failed to scan model", zap.Error(err))
			continue
		}

		// Parse metadata JSON
		var metadata map[string]interface{}
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &metadata); err != nil {
				g.logger.Warn("failed to parse model metadata", zap.Error(err))
				metadata = make(map[string]interface{})
			}
		} else {
			metadata = make(map[string]interface{})
		}

		modelsList = append(modelsList, ModelResponse{
			ID:                      m.ID.String(),
			Name:                    m.Name,
			Family:                  m.Family,
			Size:                    m.Size,
			Type:                    m.Type,
			ContextLength:           m.ContextLength,
			VRAMRequiredGB:          m.VRAMRequiredGB,
			PriceInputPerMillion:    m.PriceInputPerMillion,
			PriceOutputPerMillion:   m.PriceOutputPerMillion,
			TokensPerSecondCapacity: m.TokensPerSecondCapacity,
			Status:                  m.Status,
			Metadata:                metadata,
			CreatedAt:               m.CreatedAt,
			UpdatedAt:               m.UpdatedAt,
		})
	}

	// Build response
	response := map[string]interface{}{
		"data": modelsList,
		"pagination": PaginationResponse{
			Total:   total,
			Limit:   limit,
			Offset:  offset,
			HasMore: offset+limit < total,
		},
	}

	g.writeJSON(w, http.StatusOK, response)
}

// HandleGetModel handles GET /api/v1/admin/models/{id}
func (g *Gateway) HandleGetModel(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	modelIDStr := chi.URLParam(r, "id")

	// Parse UUID
	modelID, err := uuid.Parse(modelIDStr)
	if err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid model ID format")
		return
	}

	// Query model
	var m models.Model
	var metadataJSON []byte

	query := `
		SELECT id, name, family, size, type, context_length,
		       vram_required_gb, price_input_per_million, price_output_per_million,
		       tokens_per_second_capacity, status, metadata, created_at, updated_at
		FROM models
		WHERE id = $1
	`

	err = g.db.Pool.QueryRow(ctx, query, modelID).Scan(
		&m.ID, &m.Name, &m.Family, &m.Size, &m.Type, &m.ContextLength,
		&m.VRAMRequiredGB, &m.PriceInputPerMillion, &m.PriceOutputPerMillion,
		&m.TokensPerSecondCapacity, &m.Status, &metadataJSON, &m.CreatedAt, &m.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			g.writeError(w, http.StatusNotFound, "model not found")
			return
		}
		g.logger.Error("failed to query model", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to query model")
		return
	}

	// Parse metadata
	var metadata map[string]interface{}
	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &metadata); err != nil {
			g.logger.Warn("failed to parse model metadata", zap.Error(err))
			metadata = make(map[string]interface{})
		}
	} else {
		metadata = make(map[string]interface{})
	}

	response := ModelResponse{
		ID:                      m.ID.String(),
		Name:                    m.Name,
		Family:                  m.Family,
		Size:                    m.Size,
		Type:                    m.Type,
		ContextLength:           m.ContextLength,
		VRAMRequiredGB:          m.VRAMRequiredGB,
		PriceInputPerMillion:    m.PriceInputPerMillion,
		PriceOutputPerMillion:   m.PriceOutputPerMillion,
		TokensPerSecondCapacity: m.TokensPerSecondCapacity,
		Status:                  m.Status,
		Metadata:                metadata,
		CreatedAt:               m.CreatedAt,
		UpdatedAt:               m.UpdatedAt,
	}

	g.writeJSON(w, http.StatusOK, response)
}

// HandleCreateModel handles POST /api/v1/admin/models
func (g *Gateway) HandleCreateModel(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse request body
	var req ModelCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate required fields
	if err := validateModelCreate(&req); err != nil {
		g.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Set default status
	if req.Status == "" {
		req.Status = "active"
	}

	// Serialize metadata
	var metadataJSON []byte
	var err error
	if req.Metadata != nil {
		metadataJSON, err = json.Marshal(req.Metadata)
		if err != nil {
			g.writeError(w, http.StatusBadRequest, "invalid metadata format")
			return
		}
	} else {
		metadataJSON = []byte("{}")
	}

	// Insert model
	query := `
		INSERT INTO models (
			name, family, size, type, context_length, vram_required_gb,
			price_input_per_million, price_output_per_million, tokens_per_second_capacity,
			status, metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, created_at, updated_at
	`

	var modelID uuid.UUID
	var createdAt, updatedAt time.Time

	err = g.db.Pool.QueryRow(ctx, query,
		req.Name, req.Family, req.Size, req.Type, req.ContextLength, req.VRAMRequiredGB,
		req.PriceInputPerMillion, req.PriceOutputPerMillion, req.TokensPerSecondCapacity,
		req.Status, metadataJSON,
	).Scan(&modelID, &createdAt, &updatedAt)

	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			g.writeError(w, http.StatusConflict, "model with this name already exists")
			return
		}
		g.logger.Error("failed to create model", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to create model")
		return
	}

	g.logger.Info("model created successfully",
		zap.String("model_id", modelID.String()),
		zap.String("name", req.Name),
	)

	// Return created model
	response := ModelResponse{
		ID:                      modelID.String(),
		Name:                    req.Name,
		Family:                  req.Family,
		Size:                    req.Size,
		Type:                    req.Type,
		ContextLength:           req.ContextLength,
		VRAMRequiredGB:          req.VRAMRequiredGB,
		PriceInputPerMillion:    req.PriceInputPerMillion,
		PriceOutputPerMillion:   req.PriceOutputPerMillion,
		TokensPerSecondCapacity: req.TokensPerSecondCapacity,
		Status:                  req.Status,
		Metadata:                req.Metadata,
		CreatedAt:               createdAt,
		UpdatedAt:               updatedAt,
	}

	g.writeJSON(w, http.StatusCreated, response)
}

// HandleUpdateModel handles PUT /api/v1/admin/models/{id}
func (g *Gateway) HandleUpdateModel(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	modelIDStr := chi.URLParam(r, "id")

	// Parse UUID
	modelID, err := uuid.Parse(modelIDStr)
	if err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid model ID format")
		return
	}

	// Parse request body
	var req ModelUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate required fields
	if err := validateModelUpdate(&req); err != nil {
		g.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Serialize metadata
	var metadataJSON []byte
	if req.Metadata != nil {
		metadataJSON, err = json.Marshal(req.Metadata)
		if err != nil {
			g.writeError(w, http.StatusBadRequest, "invalid metadata format")
			return
		}
	} else {
		metadataJSON = []byte("{}")
	}

	// Update model
	query := `
		UPDATE models SET
			name = $1, family = $2, size = $3, type = $4, context_length = $5,
			vram_required_gb = $6, price_input_per_million = $7, price_output_per_million = $8,
			tokens_per_second_capacity = $9, status = $10, metadata = $11, updated_at = NOW()
		WHERE id = $12
		RETURNING updated_at
	`

	var updatedAt time.Time
	err = g.db.Pool.QueryRow(ctx, query,
		req.Name, req.Family, req.Size, req.Type, req.ContextLength, req.VRAMRequiredGB,
		req.PriceInputPerMillion, req.PriceOutputPerMillion, req.TokensPerSecondCapacity,
		req.Status, metadataJSON, modelID,
	).Scan(&updatedAt)

	if err != nil {
		if err == pgx.ErrNoRows {
			g.writeError(w, http.StatusNotFound, "model not found")
			return
		}
		g.logger.Error("failed to update model", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to update model")
		return
	}

	g.logger.Info("model updated successfully", zap.String("model_id", modelID.String()))

	// Return updated model (fetch it to get created_at)
	g.HandleGetModel(w, r)
}

// HandlePatchModel handles PATCH /api/v1/admin/models/{id}
func (g *Gateway) HandlePatchModel(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	modelIDStr := chi.URLParam(r, "id")

	// Parse UUID
	modelID, err := uuid.Parse(modelIDStr)
	if err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid model ID format")
		return
	}

	// Parse request body
	var req ModelPatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Build dynamic update query
	updates := []string{}
	args := []interface{}{}
	argIndex := 1

	if req.Name != nil {
		updates = append(updates, fmt.Sprintf("name = $%d", argIndex))
		args = append(args, *req.Name)
		argIndex++
	}

	if req.Family != nil {
		updates = append(updates, fmt.Sprintf("family = $%d", argIndex))
		args = append(args, *req.Family)
		argIndex++
	}

	if req.Size != nil {
		updates = append(updates, fmt.Sprintf("size = $%d", argIndex))
		args = append(args, *req.Size)
		argIndex++
	}

	if req.Type != nil {
		updates = append(updates, fmt.Sprintf("type = $%d", argIndex))
		args = append(args, *req.Type)
		argIndex++
	}

	if req.ContextLength != nil {
		updates = append(updates, fmt.Sprintf("context_length = $%d", argIndex))
		args = append(args, *req.ContextLength)
		argIndex++
	}

	if req.VRAMRequiredGB != nil {
		updates = append(updates, fmt.Sprintf("vram_required_gb = $%d", argIndex))
		args = append(args, *req.VRAMRequiredGB)
		argIndex++
	}

	if req.PriceInputPerMillion != nil {
		updates = append(updates, fmt.Sprintf("price_input_per_million = $%d", argIndex))
		args = append(args, *req.PriceInputPerMillion)
		argIndex++
	}

	if req.PriceOutputPerMillion != nil {
		updates = append(updates, fmt.Sprintf("price_output_per_million = $%d", argIndex))
		args = append(args, *req.PriceOutputPerMillion)
		argIndex++
	}

	if req.TokensPerSecondCapacity != nil {
		updates = append(updates, fmt.Sprintf("tokens_per_second_capacity = $%d", argIndex))
		args = append(args, *req.TokensPerSecondCapacity)
		argIndex++
	}

	if req.Status != nil {
		updates = append(updates, fmt.Sprintf("status = $%d", argIndex))
		args = append(args, *req.Status)
		argIndex++
	}

	if req.Metadata != nil {
		metadataJSON, err := json.Marshal(*req.Metadata)
		if err != nil {
			g.writeError(w, http.StatusBadRequest, "invalid metadata format")
			return
		}
		updates = append(updates, fmt.Sprintf("metadata = $%d", argIndex))
		args = append(args, metadataJSON)
		argIndex++
	}

	if len(updates) == 0 {
		g.writeError(w, http.StatusBadRequest, "no fields to update")
		return
	}

	// Always update updated_at
	updates = append(updates, "updated_at = NOW()")

	// Add model ID to args
	args = append(args, modelID)

	// Execute update
	query := fmt.Sprintf(
		"UPDATE models SET %s WHERE id = $%d RETURNING updated_at",
		strings.Join(updates, ", "),
		argIndex,
	)

	var updatedAt time.Time
	err = g.db.Pool.QueryRow(ctx, query, args...).Scan(&updatedAt)

	if err != nil {
		if err == pgx.ErrNoRows {
			g.writeError(w, http.StatusNotFound, "model not found")
			return
		}
		g.logger.Error("failed to patch model", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to patch model")
		return
	}

	g.logger.Info("model patched successfully", zap.String("model_id", modelID.String()))

	// Return updated model
	g.HandleGetModel(w, r)
}

// HandleDeleteModel handles DELETE /api/v1/admin/models/{id}
func (g *Gateway) HandleDeleteModel(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	modelIDStr := chi.URLParam(r, "id")

	// Parse UUID
	modelID, err := uuid.Parse(modelIDStr)
	if err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid model ID format")
		return
	}

	// Check if model is in use by any nodes
	var nodeCount int
	checkQuery := "SELECT COUNT(*) FROM nodes WHERE model_name IN (SELECT name FROM models WHERE id = $1)"
	err = g.db.Pool.QueryRow(ctx, checkQuery, modelID).Scan(&nodeCount)
	if err != nil {
		g.logger.Error("failed to check model usage", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to check model usage")
		return
	}

	if nodeCount > 0 {
		g.writeError(w, http.StatusConflict, fmt.Sprintf("model is in use by %d node(s) and cannot be deleted", nodeCount))
		return
	}

	// Delete model
	query := "DELETE FROM models WHERE id = $1"
	result, err := g.db.Pool.Exec(ctx, query, modelID)
	if err != nil {
		g.logger.Error("failed to delete model", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to delete model")
		return
	}

	if result.RowsAffected() == 0 {
		g.writeError(w, http.StatusNotFound, "model not found")
		return
	}

	g.logger.Info("model deleted successfully", zap.String("model_id", modelID.String()))

	w.WriteHeader(http.StatusNoContent)
}

// HandleSearchModels handles GET /api/v1/admin/models/search
func (g *Gateway) HandleSearchModels(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query parameters
	q := r.URL.Query().Get("q")
	families := r.URL.Query().Get("families")
	types := r.URL.Query().Get("types")
	minContextLength := parseIntParam(r, "min_context_length", 0, 0, 999999)
	maxPriceInput := parseFloatParam(r, "max_price_input", 0)
	limit := parseIntParam(r, "limit", 50, 1, 100)
	offset := parseIntParam(r, "offset", 0, 0, 999999)

	// Build query
	queryBuilder := strings.Builder{}
	queryBuilder.WriteString(`
		SELECT id, name, family, size, type, context_length,
		       vram_required_gb, price_input_per_million, price_output_per_million,
		       tokens_per_second_capacity, status, metadata, created_at, updated_at
		FROM models
		WHERE 1=1
	`)

	countBuilder := strings.Builder{}
	countBuilder.WriteString("SELECT COUNT(*) FROM models WHERE 1=1")

	args := []interface{}{}
	argIndex := 1

	// Add search query
	if q != "" {
		searchCondition := fmt.Sprintf(
			" AND (name ILIKE $%d OR family ILIKE $%d OR metadata::text ILIKE $%d)",
			argIndex, argIndex, argIndex,
		)
		queryBuilder.WriteString(searchCondition)
		countBuilder.WriteString(searchCondition)
		args = append(args, "%"+q+"%")
		argIndex++
	}

	// Add families filter
	if families != "" {
		familyList := strings.Split(families, ",")
		placeholders := []string{}
		for _, family := range familyList {
			placeholders = append(placeholders, fmt.Sprintf("$%d", argIndex))
			args = append(args, strings.TrimSpace(family))
			argIndex++
		}
		condition := fmt.Sprintf(" AND family IN (%s)", strings.Join(placeholders, ","))
		queryBuilder.WriteString(condition)
		countBuilder.WriteString(condition)
	}

	// Add types filter
	if types != "" {
		typeList := strings.Split(types, ",")
		placeholders := []string{}
		for _, t := range typeList {
			placeholders = append(placeholders, fmt.Sprintf("$%d", argIndex))
			args = append(args, strings.TrimSpace(t))
			argIndex++
		}
		condition := fmt.Sprintf(" AND type IN (%s)", strings.Join(placeholders, ","))
		queryBuilder.WriteString(condition)
		countBuilder.WriteString(condition)
	}

	// Add context length filter
	if minContextLength > 0 {
		condition := fmt.Sprintf(" AND context_length >= $%d", argIndex)
		queryBuilder.WriteString(condition)
		countBuilder.WriteString(condition)
		args = append(args, minContextLength)
		argIndex++
	}

	// Add price filter
	if maxPriceInput > 0 {
		condition := fmt.Sprintf(" AND price_input_per_million <= $%d", argIndex)
		queryBuilder.WriteString(condition)
		countBuilder.WriteString(condition)
		args = append(args, maxPriceInput)
		argIndex++
	}

	// Add ordering
	queryBuilder.WriteString(" ORDER BY name ASC")

	// Add pagination
	queryBuilder.WriteString(fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIndex, argIndex+1))
	args = append(args, limit, offset)

	// Get total count
	var total int
	err := g.db.Pool.QueryRow(ctx, countBuilder.String(), args[:len(args)-2]...).Scan(&total)
	if err != nil {
		g.logger.Error("failed to count models", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to count models")
		return
	}

	// Execute query
	rows, err := g.db.Pool.Query(ctx, queryBuilder.String(), args...)
	if err != nil {
		g.logger.Error("failed to search models", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to search models")
		return
	}
	defer rows.Close()

	modelsList := []ModelResponse{}
	for rows.Next() {
		var m models.Model
		var metadataJSON []byte

		err := rows.Scan(
			&m.ID, &m.Name, &m.Family, &m.Size, &m.Type, &m.ContextLength,
			&m.VRAMRequiredGB, &m.PriceInputPerMillion, &m.PriceOutputPerMillion,
			&m.TokensPerSecondCapacity, &m.Status, &metadataJSON, &m.CreatedAt, &m.UpdatedAt,
		)
		if err != nil {
			g.logger.Error("failed to scan model", zap.Error(err))
			continue
		}

		var metadata map[string]interface{}
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &metadata); err != nil {
				metadata = make(map[string]interface{})
			}
		} else {
			metadata = make(map[string]interface{})
		}

		modelsList = append(modelsList, ModelResponse{
			ID:                      m.ID.String(),
			Name:                    m.Name,
			Family:                  m.Family,
			Size:                    m.Size,
			Type:                    m.Type,
			ContextLength:           m.ContextLength,
			VRAMRequiredGB:          m.VRAMRequiredGB,
			PriceInputPerMillion:    m.PriceInputPerMillion,
			PriceOutputPerMillion:   m.PriceOutputPerMillion,
			TokensPerSecondCapacity: m.TokensPerSecondCapacity,
			Status:                  m.Status,
			Metadata:                metadata,
			CreatedAt:               m.CreatedAt,
			UpdatedAt:               m.UpdatedAt,
		})
	}

	response := map[string]interface{}{
		"data": modelsList,
		"pagination": PaginationResponse{
			Total:   total,
			Limit:   limit,
			Offset:  offset,
			HasMore: offset+limit < total,
		},
		"query": q,
	}

	g.writeJSON(w, http.StatusOK, response)
}

// Validation functions

func validateModelCreate(req *ModelCreateRequest) error {
	if req.Name == "" {
		return fmt.Errorf("name is required")
	}
	if req.Family == "" {
		return fmt.Errorf("family is required")
	}
	if req.Type == "" {
		return fmt.Errorf("type is required")
	}
	if req.Type != "completion" && req.Type != "chat" && req.Type != "embedding" {
		return fmt.Errorf("type must be 'completion', 'chat', or 'embedding'")
	}
	if req.ContextLength <= 0 {
		return fmt.Errorf("context_length must be positive")
	}
	if req.VRAMRequiredGB <= 0 {
		return fmt.Errorf("vram_required_gb must be positive")
	}
	if req.PriceInputPerMillion < 0 {
		return fmt.Errorf("price_input_per_million must be non-negative")
	}
	if req.PriceOutputPerMillion < 0 {
		return fmt.Errorf("price_output_per_million must be non-negative")
	}
	if req.Status != "" && req.Status != "active" && req.Status != "deprecated" && req.Status != "beta" {
		return fmt.Errorf("status must be 'active', 'deprecated', or 'beta'")
	}
	return nil
}

func validateModelUpdate(req *ModelUpdateRequest) error {
	if req.Name == "" {
		return fmt.Errorf("name is required")
	}
	if req.Family == "" {
		return fmt.Errorf("family is required")
	}
	if req.Type == "" {
		return fmt.Errorf("type is required")
	}
	if req.Type != "completion" && req.Type != "chat" && req.Type != "embedding" {
		return fmt.Errorf("type must be 'completion', 'chat', or 'embedding'")
	}
	if req.ContextLength <= 0 {
		return fmt.Errorf("context_length must be positive")
	}
	if req.VRAMRequiredGB <= 0 {
		return fmt.Errorf("vram_required_gb must be positive")
	}
	if req.PriceInputPerMillion < 0 {
		return fmt.Errorf("price_input_per_million must be non-negative")
	}
	if req.PriceOutputPerMillion < 0 {
		return fmt.Errorf("price_output_per_million must be non-negative")
	}
	if req.Status != "active" && req.Status != "deprecated" && req.Status != "beta" {
		return fmt.Errorf("status must be 'active', 'deprecated', or 'beta'")
	}
	return nil
}

// Helper functions

func parseIntParam(r *http.Request, name string, defaultVal, min, max int) int {
	val := r.URL.Query().Get(name)
	if val == "" {
		return defaultVal
	}
	parsed, err := strconv.Atoi(val)
	if err != nil {
		return defaultVal
	}
	if parsed < min {
		return min
	}
	if parsed > max {
		return max
	}
	return parsed
}

func parseFloatParam(r *http.Request, name string, defaultVal float64) float64 {
	val := r.URL.Query().Get(name)
	if val == "" {
		return defaultVal
	}
	parsed, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return defaultVal
	}
	return parsed
}
