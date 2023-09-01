package plugin

import "github.com/redhat-openshift-ecosystem/openshift-preflight/x/plugin/v0"

// Register is an alias for the current version's registration function.
// Note(Jose): In theory, we don't need to use this, but I'm adding it for now
// until we decide if versioning our plugin code is something we want to pursue.
var Register = plugin.Register
