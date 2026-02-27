package api

import (
	"fmt"
	"strings"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
)

// reservedParams are query parameters that are NOT column filters.
var reservedParams = map[string]bool{
	"select": true,
	"order":  true,
	"limit":  true,
	"offset": true,
}

// ApplyFilters parses PostgREST-style query parameters and applies them to a squirrel SelectBuilder.
//
// Supported operators: eq, neq, gt, gte, lt, lte, like, ilike, in, is
// Supported meta: order, limit, offset, select
//
// Examples:
//
//	?name=eq.Naruto           → WHERE name = 'Naruto'
//	?episodes=gt.10           → WHERE episodes > 10
//	?name=like.%Nar%          → WHERE name LIKE '%Nar%'
//	?status=in.(active,draft) → WHERE status IN ('active','draft')
//	?deleted_at=is.null       → WHERE deleted_at IS NULL
//	?order=created_at.desc    → ORDER BY created_at DESC
//	?limit=10&offset=5        → LIMIT 10 OFFSET 5
//	?select=id,name           → SELECT id, name (instead of *)
func ApplyFilters(q squirrel.SelectBuilder, params map[string][]string, table string) squirrel.SelectBuilder {
	// Handle select projection
	if selectCols, ok := params["select"]; ok && len(selectCols) > 0 {
		raw := selectCols[0]

		// Separate column projections from embedded relations
		var cols []string
		for _, part := range strings.Split(raw, ",") {
			part = strings.TrimSpace(part)
			// Skip embedded relation syntax like tasks(*) — handled elsewhere
			if strings.Contains(part, "(") {
				continue
			}
			if part != "" {
				cols = append(cols, part)
			}
		}

		if len(cols) > 0 {
			sanitizedCols := make([]string, len(cols))
			for i, c := range cols {
				if c == "*" {
					sanitizedCols[i] = "*"
				} else {
					sanitizedCols[i] = pgx.Identifier{c}.Sanitize()
				}
			}
			q = q.Columns(sanitizedCols...)
		}
	}

	// Handle column filters
	for key, values := range params {
		if reservedParams[key] || len(values) == 0 {
			continue
		}

		val := values[0]

		// Parse operator.value format
		dotIdx := strings.Index(val, ".")
		if dotIdx == -1 {
			continue // No operator prefix, skip
		}

		op := val[:dotIdx]
		operand := val[dotIdx+1:]

		sKey := pgx.Identifier{key}.Sanitize()

		switch op {
		case "eq":
			q = q.Where(squirrel.Eq{sKey: operand})
		case "neq":
			q = q.Where(squirrel.NotEq{sKey: operand})
		case "gt":
			q = q.Where(squirrel.Gt{sKey: operand})
		case "gte":
			q = q.Where(squirrel.GtOrEq{sKey: operand})
		case "lt":
			q = q.Where(squirrel.Lt{sKey: operand})
		case "lte":
			q = q.Where(squirrel.LtOrEq{sKey: operand})
		case "like":
			q = q.Where(fmt.Sprintf("%s LIKE ?", sKey), operand)
		case "ilike":
			q = q.Where(fmt.Sprintf("%s ILIKE ?", sKey), operand)
		case "in":
			// Parse (val1,val2,val3) format
			operand = strings.TrimPrefix(operand, "(")
			operand = strings.TrimSuffix(operand, ")")
			parts := strings.Split(operand, ",")
			ifaces := make([]interface{}, len(parts))
			for i, p := range parts {
				ifaces[i] = strings.TrimSpace(p)
			}
			q = q.Where(squirrel.Eq{sKey: ifaces})
		case "is":
			if strings.ToLower(operand) == "null" {
				q = q.Where(fmt.Sprintf("%s IS NULL", sKey))
			} else if strings.ToLower(operand) == "true" {
				q = q.Where(squirrel.Eq{sKey: true})
			} else if strings.ToLower(operand) == "false" {
				q = q.Where(squirrel.Eq{sKey: false})
			}
		}
	}

	// Handle ordering: ?order=created_at.desc or ?order=name.asc
	if orderVals, ok := params["order"]; ok && len(orderVals) > 0 {
		for _, orderSpec := range strings.Split(orderVals[0], ",") {
			orderSpec = strings.TrimSpace(orderSpec)
			parts := strings.SplitN(orderSpec, ".", 2)
			col := parts[0]
			dir := "ASC"
			if len(parts) == 2 && strings.ToUpper(parts[1]) == "DESC" {
				dir = "DESC"
			}
			q = q.OrderBy(fmt.Sprintf("%s %s", pgx.Identifier{col}.Sanitize(), dir))
		}
	}

	// Handle limit
	if limitVals, ok := params["limit"]; ok && len(limitVals) > 0 {
		var limit uint64
		if _, err := fmt.Sscanf(limitVals[0], "%d", &limit); err == nil {
			q = q.Limit(limit)
		}
	}

	// Handle offset
	if offsetVals, ok := params["offset"]; ok && len(offsetVals) > 0 {
		var offset uint64
		if _, err := fmt.Sscanf(offsetVals[0], "%d", &offset); err == nil {
			q = q.Offset(offset)
		}
	}

	return q
}

// ParseEmbeddedRelations extracts relation embedding requests from the select parameter.
// e.g. ?select=*,tasks(*) returns ["tasks"]
func ParseEmbeddedRelations(params map[string][]string) []string {
	selectVals, ok := params["select"]
	if !ok || len(selectVals) == 0 {
		return nil
	}

	var relations []string
	for _, part := range strings.Split(selectVals[0], ",") {
		part = strings.TrimSpace(part)
		if idx := strings.Index(part, "("); idx > 0 {
			relName := part[:idx]
			relations = append(relations, relName)
		}
	}
	return relations
}
