#include <cuda.h>
#include <stdio.h>

int main() {
    printf("[TEST] Starting CUDA test...\n");

    void* ptr = nullptr;
    size_t size = 1024 * 1024;  // 1MB

    cudaError_t err = cudaMalloc(&ptr, size);

    if (err == cudaSuccess) {
        printf("[TEST] Allocated %zu bytes at %p\n", size, ptr);
    } else {
        printf("[TEST] Allocation failed: %d\n", err);
    }

    return 0;
}
