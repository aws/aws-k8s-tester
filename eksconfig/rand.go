package eksconfig

import (
	"encoding/hex"
	"math/rand"
	"time"
)

const ll = "0123456789abcdefghijklmnopqrstuvwxyz"

func randString(n int) string {
	b := make([]byte, n)
	for i := range b {
		rand.Seed(time.Now().UnixNano())
		b[i] = ll[rand.Intn(len(ll))]
	}
	pfx := randoms[rand.Intn(len(randoms))]
	s := pfx + string(b)
	if len(s) > n {
		s = s[:n]
	}
	return s
}

func randBytes(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		rand.Seed(time.Now().UnixNano())
		b[i] = ll[rand.Intn(len(ll))]
	}
	pfx := randoms[rand.Intn(len(randoms))]
	s := pfx + string(b)
	if len(s) > n {
		s = s[:n]
	}
	return []byte(s)
}

// openssl rand -hex 32
func randHex(n int) string {
	return hex.EncodeToString(randBytes(n))
}

var randoms = []string{
	"autumn",
	"sun",
	"splendid",
	"sunny",
	"original",
	"dream",
	"whole",
	"aws",
	"amazon",
	"flow",
	"cherry",
	"tree",
	"frost",
	"morning",
	"grand",
	"sparkling",
	"wandering",
	"snowy",
	"summertime",
	"butterfly",
	"boldly",
	"green",
	"river",
	"breeze",
	"hiking",
	"proud",
	"floral",
	"divine",
	"modern",
	"delight",
	"lively",
	"forte",
	"waterfall",
	"embark",
	"flower",
	"roadtrip",
	"atlas",
	"grass",
	"haze",
	"glacial",
	"mountain",
	"snowflake",
	"misty",
	"summer",
	"good",
	"icy",
	"coffee",
	"awesome",
	"spring",
	"twilight",
	"blue",
	"coral",
	"everest",
	"galaxy",
	"hello",
	"seattle",
	"wind",
	"watermelon",
	"sea",
	"ocean",
	"kirkland",
	"bellevue",
	"sunrise",
	"magnificent",
	"exclusive",
	"tropical",
	"morning",
	"sunset",
	"blueshift",
	"dynamic",
	"leaf",
	"forest",
	"impressive",
	"amelia",
	"amzn",
	"rufus",
	"spheres",
	"innovation",
	"apple",
	"inventive",
	"brazil",
	"milan",
	"cloud",
	"rustc",
	"sun",
	"sound",
	"sky",
	"surf",
	"island",
	"water",
	"wildflower",
	"wave",
	"charisma",
	"water",
	"amber",
	"reinvent",
	"oscar",
	"integrity",
	"accountable",
	"day1",
	"prime",
	"nitro",
	"maria",
	"frosty",
	"paper",
	"star",
	"onion",
	"linux",
	"rust",
	"hawaii",
	"otter",
	"varzea",
	"obidos",
}
