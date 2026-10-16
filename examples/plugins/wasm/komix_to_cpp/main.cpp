// Minimal WASM C++ plugin for testing host<->guest calling
// Build: zig cc -target wasm32-freestanding -O2 main.cpp -o main.wasm -nostdlib
//
// All functions export with C ABI (export_name attribute for host to find).
// Freestanding C++: no stdlib, custom bump allocator.

#include <stdint.h>

// Freestanding C++: <cstdint> not available without stdlib. <stdint.h> works
// and provides the integer types used in this plugin.

// Host imports (env module)
// zig cc's clang adds 'env.' prefix to import_name automatically, so use 'host_call' only
extern uint64_t host_call(uint32_t ptr, uint32_t len) __attribute__((import_name("host_call")));

// Guest memory: simple bump allocator
static uint8_t heap[512 * 1024];
static uint32_t heap_off = 0;

extern "C" {

uint32_t malloc(uint32_t size) {
    if (size == 0) size = 1;
    if (heap_off + size > (uint32_t)sizeof(heap)) return 0;
    uint8_t* p = heap + heap_off;
    heap_off += size;
    return (uint32_t)(uintptr_t)p;
}

void free(uint32_t ptr) { (void)ptr; }

// Copy string with bounded loop (freestanding: no string.h)
static void copy_str(char* dst, const char* src, uint32_t max) {
    uint32_t i = 0;
    while (i < max && src[i]) { dst[i] = src[i]; i++; }
    dst[i] = '\0';
}

// ABI exports
__attribute__((export_name("contract_version")))
int32_t contract_version() { return 1; }

__attribute__((export_name("Init")))
uint64_t Init(uint32_t in_ptr, uint32_t in_len) {
    const char* input = (const char*)(uintptr_t)in_ptr;
    (void)input;

    const char* log_msg = "komix_to wasm Init called";
    uint32_t lp = malloc(64);
    if (lp) {
        copy_str((char*)(uintptr_t)lp, log_msg, 64);
        host_call(lp, 22);
    }

    const char* result = "{\"id\":\"komix_to\",\"kind\":\"wasm\",\"name\":\"Komix To\"}";
    uint32_t rlen = 52;
    uint32_t rp = malloc(rlen + 1);
    if (!rp) return 0;
    copy_str((char*)(uintptr_t)rp, result, rlen + 1);
    return ((uint64_t)rlen << 32) | rp;
}

__attribute__((export_name("Search")))
uint64_t Search(uint32_t in_ptr, uint32_t in_len) {
    const char* input = (const char*)(uintptr_t)in_ptr;
    (void)input;

    const char* log_msg = "komix_to Search called";
    uint32_t lp = malloc(64);
    if (lp) {
        copy_str((char*)(uintptr_t)lp, log_msg, 64);
        host_call(lp, 22);
    }

    const char* result = "[]";
    uint32_t rp = malloc(3);
    if (!rp) return 0;
    copy_str((char*)(uintptr_t)rp, result, 3);
    return ((uint64_t)2 << 32) | rp;
}

__attribute__((export_name("GetMangaDetail")))
uint64_t GetMangaDetail(uint32_t in_ptr, uint32_t in_len) {
    const char* input = (const char*)(uintptr_t)in_ptr;
    (void)input;

    const char* log_msg = "komix_to GetMangaDetail called";
    uint32_t lp = malloc(64);
    if (lp) {
        copy_str((char*)(uintptr_t)lp, log_msg, 64);
        host_call(lp, 28);
    }

    const char* result = "{}";
    uint32_t rp = malloc(3);
    if (!rp) return 0;
    copy_str((char*)(uintptr_t)rp, result, 3);
    return ((uint64_t)2 << 32) | rp;
}

__attribute__((export_name("GetChapterList")))
uint64_t GetChapterList(uint32_t in_ptr, uint32_t in_len) {
    const char* input = (const char*)(uintptr_t)in_ptr;
    (void)input;

    const char* log_msg = "komix_to GetChapterList called";
    uint32_t lp = malloc(64);
    if (lp) {
        copy_str((char*)(uintptr_t)lp, log_msg, 64);
        host_call(lp, 29);
    }

    const char* result = "[]";
    uint32_t rp = malloc(3);
    if (!rp) return 0;
    copy_str((char*)(uintptr_t)rp, result, 3);
    return ((uint64_t)2 << 32) | rp;
}

__attribute__((export_name("GetPageList")))
uint64_t GetPageList(uint32_t in_ptr, uint32_t in_len) {
    const char* input = (const char*)(uintptr_t)in_ptr;
    (void)input;

    const char* log_msg = "komix_to GetPageList called";
    uint32_t lp = malloc(64);
    if (lp) {
        copy_str((char*)(uintptr_t)lp, log_msg, 64);
        host_call(lp, 26);
    }

    const char* result = "[]";
    uint32_t rp = malloc(3);
    if (!rp) return 0;
    copy_str((char*)(uintptr_t)rp, result, 3);
    return ((uint64_t)2 << 32) | rp;
}

__attribute__((export_name("GetGenres")))
uint64_t GetGenres(uint32_t in_ptr, uint32_t in_len) {
    const char* input = (const char*)(uintptr_t)in_ptr;
    (void)input;

    const char* log_msg = "komix_to GetGenres called";
    uint32_t lp = malloc(64);
    if (lp) {
        copy_str((char*)(uintptr_t)lp, log_msg, 64);
        host_call(lp, 24);
    }

    const char* result = "[]";
    uint32_t rp = malloc(3);
    if (!rp) return 0;
    copy_str((char*)(uintptr_t)rp, result, 3);
    return ((uint64_t)2 << 32) | rp;
}

} // extern "C"
