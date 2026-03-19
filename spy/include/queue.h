#ifndef COMPUTE_SENTRY_SPY_QUEUE_H
#define COMPUTE_SENTRY_SPY_QUEUE_H

#include <atomic>
#include <memory>
#include <vector>

namespace compute_sentry {

// A simple ring-buffer based lock-free queue for producer-consumer scenarios.
// Suitable for high-frequency metrics collection.
template <typename T, size_t Capacity = 4096>
class LockFreeQueue {
public:
    LockFreeQueue() : head_(0), tail_(0) {}

    bool Push(const T& item) {
        size_t tail = tail_.load(std::memory_order_relaxed);
        size_t next_tail = (tail + 1) % Capacity;
        if (next_tail == head_.load(std::memory_order_acquire)) {
            return false; // Queue full
        }
        buffer_[tail] = item;
        tail_.store(next_tail, std::memory_order_release);
        return true;
    }

    bool Pop(T& item) {
        size_t head = head_.load(std::memory_order_relaxed);
        if (head == tail_.load(std::memory_order_acquire)) {
            return false; // Queue empty
        }
        item = buffer_[head];
        head_.store((head + 1) % Capacity, std::memory_order_release);
        return true;
    }

private:
    T buffer_[Capacity];
    std::atomic<size_t> head_;
    std::atomic<size_t> tail_;
};

struct MetricEvent {
    enum class Type {
        NCCL_ALL_REDUCE,
        CUDA_MALLOC,
        CUDA_MEMCPY
    };

    Type type;
    char gpu_model[32]; // New field for precise GPU identification
    long long duration_us;
    size_t count;
};

} // namespace compute_sentry

#endif // COMPUTE_SENTRY_SPY_QUEUE_H
