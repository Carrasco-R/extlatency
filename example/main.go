package main

import (
	"github.com/Carrasco-R/extlatency"
)

func main() {
	// exampleLog := "ExtLatency: HR=0,BR=0,PS=0,AAA=2,AXF=2,AXF=2,ARE=2,BS=2,CS=2,BS=3, == HR=4,BR=4,PS=4,AXF=5,ARE=5,ARA=5,BS=5,TC=5"
	exampleLog := "ExtLatency: TS=0,HR=0,BR=0,PS=0,RT=0,PS=0,COR=0,WSDL=0,HTM=0,CI=0,RAL=0,SE=0,PC=0,FC=0,PS=0,XS=0,XSL=1,GS=2,XSL=2,GS=9,SW=9,FC=9,IV=21,XC=21,PC=21,FC=21,PS=21,GS=21,PC=21,FC=21,PC=21,PS=21,RES=21,PC=21,FC=21,HS=21,FNL=22,PC=22,BS=22,TC=22, [https://example.com]"
	extlatency.Parse(exampleLog)
}
