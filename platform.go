package niu

import "strconv"

type Platform int8

const (
	Unspecify  Platform = 0
	Android    Platform = 1
	AndroidPad Platform = 2
	IPhone     Platform = 3
	Mac        Platform = 4
	IPad       Platform = 5
	Windows    Platform = 6
	Linux      Platform = 7
	Web        Platform = 8
	Harmony    Platform = 9
)

var Platforms = []Platform{Android, AndroidPad, IPhone, IPad, Mac, IPad, Windows, Linux, Web, Harmony}

func IsPlatformValid(p Platform) bool {
	for _, v := range Platforms {
		if v == p {
			return true
		}
	}
	return false
}

func IsPlatformStringValid(pstr string) bool {
	if len(pstr) == 0 {
		return false
	}

	p, err := strconv.Atoi(pstr)
	if err != nil {
		return false
	}

	for _, v := range Platforms {
		if v == Platform(p) {
			return true
		}
	}
	return false
}
