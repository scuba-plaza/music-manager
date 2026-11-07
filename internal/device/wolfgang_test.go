package device_test

import "music-manager/internal/device"

var wolfgang = builtinTarget("wolfgang-wmp01")

func builtinTarget(name string) device.Target {
	target, ok := device.Builtin().Get(name)
	if !ok {
		panic("no built-in profile " + name)
	}
	return target
}
