package app_test

import "music-manager/internal/device"

var wolfgang = func() device.Target {
	target, ok := device.Builtin().Get("wolfgang-wmp01")
	if !ok {
		panic("no built-in profile wolfgang-wmp01")
	}
	return target
}()
