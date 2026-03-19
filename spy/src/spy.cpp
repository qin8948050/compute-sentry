#include <iostream>
#include <dlfcn.h>
#include <chrono>
#include <thread>
#include <atomic>
#include <cstring>
#include "queue.h"
#include "uds.h"

using namespace compute_sentry;

// NCCL function signature
typedef int (*ncclAllReduce_t)(const void* sendbuff, void* recvbuff, size_t count,
                               int datatype, int op, void* comm, void* stream);

static ncclAllReduce_t real_ncclAllReduce = nullptr;
static LockFreeQueue<MetricEvent> event_queue;
static std::atomic<bool> running(true);
static std::thread background_thread;

// Cache for GPU model
static char cached_gpu_model[32] = "unknown";

// Dynamically detect GPU model using CUDA Runtime API
void DetectGPUModel() {
    // 1. Check for environment override first (useful for testing or fixed-model containers)
    const char* env_override = getenv("COMPUTE_SENTRY_GPU_MODEL_OVERRIDE");
    if (env_override) {
        strncpy(cached_gpu_model, env_override, 31);
        cached_gpu_model[31] = '\0';
        return;
    }

    // 2. Try to detect via libcudart.so
    void* handle = dlopen("libcudart.so", RTLD_LAZY);
    if (!handle) return;

    typedef int (*get_device_t)(int*);
    // In cudaDeviceProp, 'name' is the first field (char name[256])
    typedef int (*get_props_t)(char*, int);

    auto get_device = (get_device_t)dlsym(handle, "cudaGetDevice");
    auto get_props = (get_props_t)dlsym(handle, "cudaGetDeviceProperties");

    if (get_device && get_props) {
        int device = 0;
        if (get_device(&device) == 0) {
            char prop_name[256]; 
            if (get_props(prop_name, device) == 0) {
                // Clean up: skip common prefixes to keep the model name concise
                const char* p = prop_name;
                if (strncmp(p, "NVIDIA ", 7) == 0) p += 7;
                if (strncmp(p, "GeForce ", 8) == 0) p += 8;
                if (strncmp(p, "Tesla ", 6) == 0) p += 6;

                strncpy(cached_gpu_model, p, 31);
                cached_gpu_model[31] = '\0';
            }
        }
    }
    dlclose(handle);
}

void BackgroundSender() {
    DetectGPUModel();
    const char* env_path = getenv("COMPUTE_SENTRY_UDS_PATH");
    std::string path = env_path ? env_path : "/var/run/compute-sentry/spy.sock";
    UDSClient client(path);
    MetricEvent event;
    while (running) {
        if (event_queue.Pop(event)) {
            client.Send(event);
        } else {
            std::this_thread::sleep_for(std::chrono::milliseconds(10));
        }
    }
}

__attribute__((constructor))
void InitSpy() {
    background_thread = std::thread(BackgroundSender);
}

__attribute__((destructor))
void DeinitSpy() {
    running = false;
    if (background_thread.joinable()) {
        background_thread.join();
    }
}

extern "C" {

int ncclAllReduce(const void* sendbuff, void* recvbuff, size_t count,
                  int datatype, int op, void* comm, void* stream) {
    if (!real_ncclAllReduce) {
        real_ncclAllReduce = (ncclAllReduce_t)dlsym(RTLD_NEXT, "ncclAllReduce");
        if (!real_ncclAllReduce) return -1;
    }

    auto start = std::chrono::high_resolution_clock::now();
    int result = real_ncclAllReduce(sendbuff, recvbuff, count, datatype, op, comm, stream);
    auto end = std::chrono::high_resolution_clock::now();
    auto duration = std::chrono::duration_cast<std::chrono::microseconds>(end - start).count();

    MetricEvent event;
    event.type = MetricEvent::Type::NCCL_ALL_REDUCE;
    strncpy(event.gpu_model, cached_gpu_model, 31);
    event.duration_us = duration;
    event.count = count;
    event_queue.Push(event);

    return result;
}

} // extern "C"
