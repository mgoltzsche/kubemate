package passwordgen

type bigram struct {
	Bigram string
	Weight int
}

type weightedChar struct {
	Char   byte
	Weight int
}
