module github.com/apache/skywalking-go/plugins/http

go 1.18

require github.com/apache/skywalking-go/plugins/core v0.0.0-20230414024435-7b292984eb80

require github.com/dave/dst v0.27.2 // indirect

replace github.com/apache/skywalking-go/plugins/core => ../core

replace github.com/apache/skywalking-go => ../../
