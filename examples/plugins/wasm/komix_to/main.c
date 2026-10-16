// Minimal WASM plugin for testing host<->guest calling
// Build: zig cc -target wasm32-freestanding -O2 main.c -o main.wasm -nostdlib
//
// All functions export with C ABI (export_name attribute for host to find).

#include <stdint.h>

// Host imports (env module)
extern uint64_t host_call(uint32_t ptr, uint32_t len) __attribute__((import_name("env.host_call")));

// Guest memory: simple bump allocator
static uint8_t heap[512 * 1024];
static uint32_t heap_off = 0;

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
    // Read host string
    const char* input = (const char*)(uintptr_t)in_ptr;

    // Log call to host via host_call
    const char* log_msg = "komix_to wasm Init called: ";
    uint32_t lp = malloc(64 + in_len);
    if (lp) {
        // Copy log prefix
        uint32_t i;
        for (i = 0; log_msg[i]; i++) ((char*)(uintptr_t)lp)[i] = log_msg[i];
        // Copy input
        for (i = 0; i < in_len; i++) ((char*)(uintptr_t)lp)[i + 64 - 1] = input[i];
        ((char*)(uintptr_t)lp)[i + 64 - 1] = '\0';
        host_call(lp, in_len + 64);
    }

    // Return plugin metadata JSON
    const char* result = "{\"id\":\"komix_to\",\"kind\":\"wasm\",\"name\":\"Komix To\"}";
    uint32_t rlen = 52;
    uint32_t rp = malloc(rlen + 1);
    if (!rp) return 0;
    uint32_t i;
    for (i = 0; i < rlen; i++) ((char*)(uintptr_t)rp)[i] = result[i];
    ((char*)(uintptr_t)rp)[rlen] = '\0';
    return ((uint64_t)rlen << 32) | rp;
}

__attribute__((export_name("Search")))
uint64_t Search(uint32_t in_ptr, uint32_t in_len) {
    const char* input = (const char*)(uintptr_t)in_ptr;
    (void)input;

    const char* log_msg = "komix_to Search called";
    uint32_t lp = malloc(64);
    if (lp) {
        uint32_t i;
        for (i = 0; log_msg[i]; i++) ((char*)(uintptr_t)lp)[i] = log_msg[i];
        host_call(lp, 22);
    }

    const char* result = "[]";
    uint32_t rp = malloc(3);
    if (!rp) return 0;
    ((char*)(uintptr_t)rp)[0] = '[';
    ((char*)(uintptr_t)rp)[1] = ']';
    ((char*)(uintptr_t)rp)[2] = '\0';
    return ((uint64_t)2 << 32) | rp;
}

__attribute__((export_name("GetMangaDetail")))
uint64_t GetMangaDetail(uint32_t in_ptr, uint32_t in_len) {
    const char* input = (const char*)(uintptr_t)in_ptr;
    (void)input;

    const char* log_msg = "komix_to GetMangaDetail called";
    uint32_t lp = malloc(64);
    if (lp) {
        uint32_t i;
        for (i = 0; log_msg[i]; i++) ((char*)(uintptr_t)lp)[i] = log_msg[i];
        host_call(lp, 28);
    }

    const char* result = "{}";
    uint32_t rp = malloc(3);
    if (!rp) return 0;
    ((char*)(uintptr_t)rp)[0] = '{';
    ((char*)(uintptr_t)rp)[1] = '}';
    ((char*)(uintptr_t)rp)[2] = '\0';
    return ((uint64_t)2 << 32) | rp;
}

__attribute__((export_name("GetChapterList")))
uint64_t GetChapterList(uint32_t in_ptr, uint32_t in_len) {
    const char* input = (const char*)(uintptr_t)in_ptr;
    (void)input;

    const char* log_msg = "komix_to GetChapterList called";
    uint32_t lp = malloc(64);
    if (lp) {
        uint32_t i;
        for (i = 0; log_msg[i]; i++) ((char*)(uintptr_t)lp)[i] = log_msg[i];
        host_call(lp, 29);
    }

    const char* result = "[]";
    uint32_t rp = malloc(3);
    if (!rp) return 0;
    ((char*)(uintptr_t)rp)[0] = '[';
    ((char*)(uintptr_t)rp)[1] = ']';
    ((char*)(uintptr_t)rp)[2] = '\0';
    return ((uint64_t)2 << 32) | rp;
}

__attribute__((export_name("GetPageList")))
uint64_t GetPageList(uint32_t in_ptr, uint32_t in_len) {
    const char* input = (const char*)(uintptr_t)in_ptr;
    (void)input;

    const char* log_msg = "komix_to GetPageList called";
    uint32_t lp = malloc(64);
    if (lp) {
        uint32_t i;
        for (i = 0; log_msg[i]; i++) ((char*)(uintptr_t)lp)[i] = log_msg[i];
        host_call(lp, 26);
    }

    const char* result = "[]";
    uint32_t rp = malloc(3);
    if (!rp) return 0;
    ((char*)(uintptr_t)rp)[0] = '[';
    ((char*)(uintptr_t)rp)[1] = ']';
    ((char*)(uintptr_t)rp)[2] = '\0';
    return ((uint64_t)2 << 32) | rp;
}

__attribute__((export_name("GetGenres")))
uint64_t GetGenres(uint32_t in_ptr, uint32_t in_len) {
    const char* input = (const char*)(uintptr_t)in_ptr;
    (void)input;

    const char* log_msg = "komix_to GetGenres called";
    uint32_t lp = malloc(64);
    if (lp) {
        uint32_t i;
        for (i = 0; log_msg[i]; i++) ((char*)(uintptr_t)lp)[i] = log_msg[i];
        host_call(lp, 24);
    }

    const char* result = "[]";
    uint32_t rp = malloc(3);
    if (!rp) return 0;
    ((char*)(uintptr_t)rp)[0] = '[';
    ((char*)(uintptr_t)rp)[1] = ']';
    ((char*)(uintptr_t)rp)[2] = '\0';
    return ((uint64_t)2 << 32) | rp;
}
