package category

type Category struct {
	ID          int64
	Name        string
	Slug        string
	Description string
	IsActive    bool
}

type ListFilter struct {
	OnlyActive bool
}
