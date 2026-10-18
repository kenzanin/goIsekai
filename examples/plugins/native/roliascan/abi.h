// abi.h — the export macro shared by every native goIsekai plugin.
#pragma once

// The host dlopen's the library and looks symbols up by name, so each entry
// point has to stay exported even though the build hides everything else.
#define EXPORT extern "C" __attribute__((visibility("default")))
