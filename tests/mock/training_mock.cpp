#include <iostream>
#include <chrono>
#include <thread>

// Function signature for ncclAllReduce (to be linked or preloaded)
extern "C" int ncclAllReduce(const void* sendbuff, void* recvbuff, size_t count,
                             int datatype, int op, void* comm, void* stream);

int main() {
    std::cout << "[Mock Training] Starting mock training process..." << std::endl;

    for (int i = 0; i < 5; ++i) {
        std::cout << "[Mock Training] Iteration " << i << ": Calling ncclAllReduce..." << std::endl;
        
        auto start = std::chrono::high_resolution_clock::now();
        ncclAllReduce(nullptr, nullptr, 1024 * 1024, 0, 0, nullptr, nullptr);
        auto end = std::chrono::high_resolution_clock::now();
        
        auto duration = std::chrono::duration_cast<std::chrono::milliseconds>(end - start).count();
        std::cout << "[Mock Training] Iteration " << i << " finished. Time: " << duration << " ms" << std::endl;
        
        std::this_thread::sleep_for(std::chrono::seconds(1));
    }

    std::cout << "[Mock Training] Training finished." << std::endl;
    return 0;
}
