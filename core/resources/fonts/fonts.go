// Package fonts contains immutable, framework-independent reading resources.
package fonts

import _ "embed"

//go:embed NotoSansSC-Regular.otf
var Regular []byte

//go:embed LICENSE
var License string

const Family = "BYR Sans"
