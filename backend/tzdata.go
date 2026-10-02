package main

// Routers (OpenWrt) and minimal containers often ship no zoneinfo; the
// embedded database keeps the panel's time zone setting working there.
import _ "time/tzdata"
