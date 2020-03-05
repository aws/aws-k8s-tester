package ec2config

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
	"autumn", "sun", "dream", "cherry", "tree", "frost", "morning", "sparkling", "wandering", "snowy", "butterfly", "boldly", "green", "river", "breeze", "proud", "floral", "divine", "polished", "ancient", "delight", "lively", "forte", "waterfall", "embark", "flower", "atlas", "grass", "haze", "glacial", "mountain", "snowflake", "misty", "summer", "icy", "siberian", "awesome", "spring", "winter", "twilight", "dawn", "blue", "coral", "bird", "everest", "galaxy", "hello", "seattle", "wind", "sea", "ocean", "sunrise", "tropical", "morning", "snow", "lake", "sunset", "blueshift", "pine", "leaf", "glitter", "forest", "amelia", "amzn", "rufus", "spheres", "maverick", "brazil", "milan", "cloud", "sun", "sound", "sky", "surf", "water", "wildflower", "wave", "charisma", "water", "amber", "reinvent", "oscar", "falling", "day1", "prime", "nitro", "frosty", "paper", "star", "onion", "hawaii", "otter", "varzea", "obidos",
}
