package pg

func Offset(page int32, pageSize int32) int32 {
	return (page - 1) * pageSize
}
