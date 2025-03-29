package niu

import (
	"slices"
	"strconv"
)

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
	return slices.Contains(Platforms, p)
}

func ParsePlatform(pstr string) Platform {
	if !IsPlatformStringValid(pstr) {
		return Unspecify
	}

	p, err := strconv.Atoi(pstr)
	if err != nil {
		return Unspecify
	}
	return Platform(p)
}

func IsPlatformStringValid(pstr string) bool {
	if len(pstr) == 0 {
		return false
	}

	p, err := strconv.Atoi(pstr)
	if err != nil {
		return false
	}

	return slices.Contains(Platforms, Platform(p))
}
