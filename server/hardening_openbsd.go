// SPDX-License-Identifier: AGPL-3.0-or-later
//go:build openbsd

package main

import "golang.org/x/sys/unix"

func hardening() {
	if err := unix.PledgePromises("stdio rpath wpath inet tty proc exec"); err != nil {
		panic(err)
	}
}
