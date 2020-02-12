package eksconfig

import (
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

var randoms = []string{
	"autumn", "resonance", "sun", "wood", "dream", "cherry", "tree", "fog", "frost", "voice", "morning", "sparkling", "wandering", "wild", "black", "holy", "snowy", "butterfly", "long", "lingering", "bold", "green", "river", "breeze", "proud", "floral", "divine", "polished", "ancient", "delight", "purple", "lively", "waterfall", "flower", "firefly", "feather", "grass", "haze", "glacial", "mountain", "snowflake", "silence", "misty", "dry", "summer", "icy", "delicate", "siberian", "cool", "spring", "winter", "patient", "twilight", "dawn", "blue", "coral", "bird", "everest", "brook", "rain", "wind", "sea", "morning", "snow", "lake", "sunset", "blueshift", "pine", "leaf", "dawn", "glitter", "forest", "milan", "cloud", "meadow", "sun", "sound", "sky", "shape", "surf", "water", "wildflower", "wave", "water", "amber", "damp", "reinvent", "falling", "day1", "prime", "nitro", "frosty", "paper", "star", "onion", "hawaii",
}
