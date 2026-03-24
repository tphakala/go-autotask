package autotask

import "encoding/json"

type Operator string

const maxRecordsLimit = 500

const (
	OpEq         Operator = "eq"
	OpNotEq      Operator = "noteq"
	OpGt         Operator = "gt"
	OpGte        Operator = "gte"
	OpLt         Operator = "lt"
	OpLte        Operator = "lte"
	OpBeginsWith Operator = "beginsWith"
	OpEndsWith   Operator = "endsWith"
	OpContains   Operator = "contains"
	OpExist      Operator = "exist"
	OpNotExist   Operator = "notExist"
	OpIn         Operator = "in"
	OpNotIn      Operator = "notIn"
)

type GroupOperator string

const (
	GroupAnd GroupOperator = "and"
	GroupOr  GroupOperator = "or"
)

type Condition interface {
	conditionNode()
}

type FieldCondition struct {
	Field string   `json:"field"`
	Op    Operator `json:"op"`
	Value any      `json:"value"`
	UDF   bool     `json:"udf,omitempty"`
}

func (FieldCondition) conditionNode() {}

type GroupCondition struct {
	Op    GroupOperator `json:"op"`
	Items []Condition   `json:"items"`
}

func (GroupCondition) conditionNode() {}

func Field(name string, op Operator, value any) FieldCondition {
	return FieldCondition{Field: name, Op: op, Value: value}
}

func UDField(name string, op Operator, value any) FieldCondition {
	return FieldCondition{Field: name, Op: op, Value: value, UDF: true}
}

func And(conditions ...Condition) GroupCondition {
	return GroupCondition{Op: GroupAnd, Items: conditions}
}

func Or(conditions ...Condition) GroupCondition {
	return GroupCondition{Op: GroupOr, Items: conditions}
}

type Query struct {
	conditions    []Condition
	includeFields []string
	maxRecords    int
}

func NewQuery() *Query { return &Query{} }

func (q *Query) Where(field string, op Operator, value any) *Query {
	q.conditions = append(q.conditions, FieldCondition{Field: field, Op: op, Value: value})
	return q
}

func (q *Query) WhereUDF(field string, op Operator, value any) *Query {
	q.conditions = append(q.conditions, FieldCondition{Field: field, Op: op, Value: value, UDF: true})
	return q
}

func (q *Query) And(conditions ...Condition) *Query {
	q.conditions = append(q.conditions, GroupCondition{Op: GroupAnd, Items: conditions})
	return q
}

func (q *Query) Or(conditions ...Condition) *Query {
	q.conditions = append(q.conditions, GroupCondition{Op: GroupOr, Items: conditions})
	return q
}

func (q *Query) Fields(fields ...string) *Query {
	q.includeFields = fields
	return q
}

func (q *Query) Limit(n int) *Query {
	q.maxRecords = n
	return q
}

func (q *Query) MaxRecords() int { return q.maxRecords }

func (q *Query) MarshalJSON() ([]byte, error) {
	m := make(map[string]any)
	if q.conditions != nil {
		m["filter"] = q.conditions
	} else {
		m["filter"] = []Condition{}
	}
	if len(q.includeFields) > 0 {
		m["IncludeFields"] = q.includeFields
	}
	if q.maxRecords > 0 {
		m["MaxRecords"] = min(q.maxRecords, maxRecordsLimit)
	}
	return json.Marshal(m)
}
