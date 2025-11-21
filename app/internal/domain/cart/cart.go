package cart

type Item struct {
	ProductID int64
	Quantity  int64
}

type DetailedItem struct {
	Item
	ProductName  string
	ProductPrice float64
}

type Cart struct {
	UserID int64
	Items  []DetailedItem
}

