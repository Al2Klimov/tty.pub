// SPDX-License-Identifier: AGPL-3.0-or-later
//go:build openbsd

package main

import "golang.org/x/sys/unix"

func hardening() {
	if err := unix.Unveil(".", "r"); err != nil {
		panic(err)
	}

	if err := unix.Unveil("/usr", "rx"); err != nil {
		panic(err)
	}

	if err := unix.Unveil("/dev/ptm", "rw"); err != nil {
		panic(err)
	}

	if err := unix.UnveilBlock(); err != nil {
		panic(err)
	}

	if err := unix.PledgePromises("stdio rpath wpath inet tty proc exec"); err != nil {
		panic(err)
	}
}
