#include <iostream>
#include <chrono>
#include <thread>

extern "C" {

int ncclAllReduce(const void* sendbuff, void* recvbuff, size_t count,
                  int datatype, int op, void* comm, void* stream) {
    // std::cout << "[Mock NCCL] ncclAllReduce called with count=" << count << std::endl;
    
    // Simulate some work/latency
    std::this_thread::sleep_for(std::chrono::milliseconds(50));
    
    return 0;
}

} // extern "C"
