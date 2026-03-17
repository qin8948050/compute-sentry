#include <dlfcn.h>
#include <stdio.h>
#include <link.h>

typedef int cudaError_t;
#define cudaSuccess 0
#define cudaErrorUnknown 999

typedef cudaError_t (*cudaMalloc_t)(void** devPtr, size_t size);

static cudaMalloc_t real_cudaMalloc = nullptr;

// LD_AUDIT 初始化
__attribute__((constructor))
static void init() {
    fprintf(stderr, "[AUDIT] hook loaded\n");

    // 尝试从 libcudart 获取真实函数
    void* handle = dlopen("libcudart.so.8.0", RTLD_NOLOAD);
    if (!handle) handle = dlopen("libcudart.so", RTLD_NOLOAD);

    if (handle) {
        real_cudaMalloc = (cudaMalloc_t)dlsym(handle, "cudaMalloc");
        if (real_cudaMalloc) {
            fprintf(stderr, "[AUDIT] Found cudaMalloc at %p\n", (void*)real_cudaMalloc);
        }
    }

    // 如果还没找到，用 RTLD_NEXT
    if (!real_cudaMalloc) {
        real_cudaMalloc = (cudaMalloc_t)dlsym(RTLD_NEXT, "cudaMalloc");
        if (real_cudaMalloc) {
            fprintf(stderr, "[AUDIT] Found cudaMalloc via RTLD_NEXT at %p\n", (void*)real_cudaMalloc);
        }
    }
}

extern "C" {

// 劫持 cudaMalloc
cudaError_t cudaMalloc(void** devPtr, size_t size) {
    if (!real_cudaMalloc) {
        fprintf(stderr, "[AUDIT] >>> cudaMalloc: REAL NOT FOUND!\n");
        return cudaErrorUnknown;
    }

    fprintf(stderr, "[AUDIT] >>> cudaMalloc: size=%zu\n", size);
    cudaError_t result = real_cudaMalloc(devPtr, size);
    fprintf(stderr, "[AUDIT] <<< cudaMalloc: result=%d, ptr=%p\n", result, *devPtr);
    return result;
}

}
