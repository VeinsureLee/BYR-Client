package desktop

import "github.com/egoist/mygo"

// appSecrets uses a fixed service name so development runs and packaged builds
// read the same saved BYR session and AI key.
var appSecrets = mygo.Secrets.Service("BYRClient")
