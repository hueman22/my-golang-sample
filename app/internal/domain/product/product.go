package product

type Product struct {
	ID          int64
	Name        string
	Description string
	Price       float64
	Stock       int64
	CategoryID  int64
	IsActive    bool
}

type ListFilter struct {
	CategoryID *int64
	Search     string
	OnlyActive bool
}

