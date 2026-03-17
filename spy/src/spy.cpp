#include <iostream>
#include <dlfcn.h>
#include <chrono>
#include <thread>
#include <atomic>
#include "queue.h"
#include "uds.h"

using namespace compute_sentry;

// NCCL function signature for ncclAllReduce
typedef int (*ncclAllReduce_t)(const void* sendbuff, void* recvbuff, size_t count,
                               int datatype, int op, void* comm, void* stream);

static ncclAllReduce_t real_ncclAllReduce = nullptr;
static LockFreeQueue<MetricEvent> event_queue;
static std::atomic<bool> running(true);
static std::thread background_thread;

void BackgroundSender() {
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

// Library constructor to start the background thread
__attribute__((constructor))
void InitSpy() {
    background_thread = std::thread(BackgroundSender);
}

// Library destructor to stop the background thread
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
        if (!real_ncclAllReduce) {
            return -1;
        }
    }

    auto start = std::chrono::high_resolution_clock::now();
    
    int result = real_ncclAllReduce(sendbuff, recvbuff, count, datatype, op, comm, stream);
    
    auto end = std::chrono::high_resolution_clock::now();
    auto duration = std::chrono::duration_cast<std::chrono::microseconds>(end - start).count();

    MetricEvent event;
    event.type = MetricEvent::Type::CUDA_MALLOC;
    event.duration_us = duration;
    event.count = count;
    event_queue.Push(event);

    return result;
}

} // extern "C"
