#include "cc/lib/hello.h"

int main() { return Hello("bazel") == "hello, bazel" ? 0 : 1; }
