// auto generated by go tool dist
// goos=linux goarch=amd64

#include "runtime.h"
void
runtime·GOMAXPROCS(int32 n, uint32, int32 ret)
{
#line 940 "/home/vsekhar/Code/go/src/github.com/vsekhar/govtil/sandbox/go/src/pkg/runtime/runtime1.goc"

	ret = runtime·gomaxprocsfunc(n);
	FLUSH(&ret);
}
void
runtime·NumCPU(int32 ret)
{
#line 944 "/home/vsekhar/Code/go/src/github.com/vsekhar/govtil/sandbox/go/src/pkg/runtime/runtime1.goc"

	ret = runtime·ncpu;
	FLUSH(&ret);
}