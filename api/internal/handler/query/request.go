package query

type queryRequest struct {
	Question   string  `json:"question" validate:"required,min=1,max=4000"`
	SnapshotID *string `json:"snapshot_id" validate:"omitempty,uuid"`
}
