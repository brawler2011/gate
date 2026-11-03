package postgres

func Offset(page int32, pageSize int32) int32 {
	return (page - 1) * pageSize
}

func ordinalToLetter(ordinal *int32) *string {
	if ordinal == nil {
		return nil
	}
	ord := int(*ordinal)
	letter := ""
	for ord >= 0 {
		letter = string(rune('A'+(ord%26))) + letter
		ord = ord/26 - 1
	}
	return &letter
}
