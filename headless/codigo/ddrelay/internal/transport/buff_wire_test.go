package transport

func buffFightCreateBody() []byte {
	player := wireJoin(wireInts(1), wireText("Zone"), wireInts(123), wireText("Player"), []byte{0, 0}, wireInts(0), []byte{1}, wireInts(0), wireText(""), wireText(""), wireText(""), wireInts(20, 0, 0, 0, 0), []byte{0}, wireInts(0), wireText(""), wireInts(0, 0, 0, 0, 0, 0, 0, 0, 0), wireText(""), wireInts(0), wireText(""), wireInts(0), []byte{0}, wireInts(0), []byte{0}, wireInts(0, 0, 0, 0, 0, 0, 1, 0, 1000, 0))
	return wireJoin([]byte{101}, wireInts(4, 0, 3, 1), player)
}
