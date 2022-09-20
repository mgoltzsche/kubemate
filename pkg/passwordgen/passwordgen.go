package passwordgen

import (
	"crypto/rand"
	"fmt"
	"math"
	"math/big"

	mathrand "math/rand"
	"strings"
	"unicode"
)

// const alphanumeric = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
const numbers = "123456789" // omit '0' to avoid confusion with 'O'

// const alphanumeric = "0123456789"
// const separatorChar = "_-/123456789"
const separatorChar = "123456789"

func GenerateMemorablePassword() (string, error) {
	return generateMemorablePassword(10)
}

func generateMemorablePassword(length int) (string, error) {
	if length < 7 {
		return "", fmt.Errorf("password length < 7 is not supported")
	}
	prefix := []byte{}
	suffix := []byte{}
	numLen := int(math.Max(1, float64(math.Min(3, float64(length/2)))))
	phraseLen := length - numLen - 1
	num := make([]byte, numLen)
	for i := 0; i < numLen; i++ {
		num[i] = numbers[randInt(len(numbers))]
	}
	if randInt(2) == 1 {
		prefix = append(num, separatorChar[randInt(len(separatorChar))])
	} else {
		suffix = append([]byte{separatorChar[randInt(len(separatorChar))]}, num...)
	}

	// Generate multiple words within one password randomly to increase entropy.
	// Guarantee a minimum amount of words.
	// (A low-entropy password is more likely to be found by brute-force even if the generator produces complex passwords otherwise eventually!)
	var phrase string
	minWordLen := 2
	maxWordLen := 7
	minWords := phraseLen / maxWordLen
	wordLengths := make([]int, minWords, minWords+1)
	wordLenSum := 0
	for i := 0; i < minWords; i++ {
		l := minWordLen + randInt(maxWordLen-minWordLen)
		wordLengths[i] = l
		wordLenSum += l
	}
	restLen := phraseLen - wordLenSum
	if restLen >= minWordLen {
		wordLengths = append(wordLengths, restLen)
	} else {
		wordLengths[minWords-1] += restLen
	}
	r := mathrand.New(cryptoRandSource{})
	r.Shuffle(len(wordLengths), func(i, j int) {
		tmp := wordLengths[i]
		wordLengths[i] = wordLengths[j]
		wordLengths[j] = tmp
	})
	for _, l := range wordLengths {
		word, err := generateWord(l)
		if err != nil {
			return "", err
		}
		if len(phrase) > 0 {
			//word = firstCharToUpper(word)
		}
		phrase += word
	}
	if randInt(3) == 0 {
		//phrase = firstCharToUpper(phrase)
	}

	/*word, err := generateWord(phraseLen)
	if err != nil {
		return "", err
	}*/
	phrase = modifyRandChar(phrase)
	return fmt.Sprintf("%s%s%s", prefix, phrase, suffix), nil
}

// generateWord generates a random, english-looking, fictional word.
// It does so based on a weighted ngram mapping that has been derived from a data set of most frequently used english words: http://norvig.com/google-books-common-words.txt
// See https://stackoverflow.com/questions/25966526/how-can-i-generate-a-random-logical-word
// See http://norvig.com/mayzner.html
// (For comparison, also see docker's name generation (combining predefined names though): https://github.com/moby/moby/blob/v20.10.17/pkg/namesgenerator/names-generator.go)
func generateWord(length int) (string, error) {
	if length < 2 {
		return "", fmt.Errorf("generate word: length must be >= 2 but provided %d", length)
	}
	bigram := randBigram()
	token := bigram.Bigram
	if length == 2 {
		return token, nil
	}
	//separator := string([]byte{symbols[randInt(len(symbols))]})
	//wordStart := 0
	for {
		lastBigram := strings.ToLower(token[len(token)-2:])
		nextChars := ngramMapping[lastBigram]
		/*if !ok { // || (multipleWords && len(token)-wordStart > 5 && len(token) < size-3 && randInt(3) == 1) {
			if !allowShorter {
				panic(fmt.Sprintf("no successor character mapped for the bigram %q", lastBigram))
			}
			return token, nil
		}*/
		nextChar := nextChars[weightedRandInt(len(nextChars), func(i int) int {
			return nextChars[i].Weight
		})]
		newToken := token + string([]byte{nextChar.Char})
		lastBigram = strings.ToLower(newToken[len(newToken)-2:])
		if _, ok := ngramMapping[lastBigram]; !ok {
			continue
		}
		token = newToken
		if len(token) == length {
			break
		}
	}
	return token, nil
}

func randBigram() bigram {
	return bigrams[weightedRandInt(len(bigrams), func(i int) int {
		return bigrams[i].Weight
	})]
}

func weightedRandInt(n int, weightFn func(i int) int) int {
	var maxWeight int
	for i := 0; i < n; i++ {
		maxWeight += weightFn(i)
	}
	k := randInt(maxWeight)
	var sum int
	for i := 0; i < n; i++ {
		sum += weightFn(i)
		if sum >= k {
			return i
		}
	}
	panic("should not happen")
}

func randInt(max int) int {
	num, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		panic(fmt.Errorf("failed to generate password: %w", err))
	}
	return int(num.Int64())
}

var substitutions = map[byte]byte{
	//'a': '/',
	'b': '6',
	//'e': '3',
	'g': '9',
	'i': '!',
	//'l': '1',
	//'o': '0',
	's': '5',
	't': '+',
}

func modifyRandChar(word string) string {
	i := randInt(len(word))
	b := []byte(word)
	if i > 0 && i < len(word)-1 {
		// try to replace char with similarly looking number or symbol
		subst, ok := substitutions[word[i]]
		if ok {
			b[i] = subst
			return string(b)
		}
	}
	b[i] = byte(unicode.ToUpper(rune(b[i])))
	return string(b)
}

func firstCharToUpper(word string) string {
	b := []byte(word)
	b[0] = byte(unicode.ToUpper(rune(b[0])))
	return string(b)
}
