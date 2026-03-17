#ifndef COMPUTE_SENTRY_SPY_UDS_H
#define COMPUTE_SENTRY_SPY_UDS_H

#include <string>
#include <sys/socket.h>
#include <sys/un.h>
#include <unistd.h>
#include "queue.h"

namespace compute_sentry {

class UDSClient {
public:
    UDSClient(const std::string& path) : path_(path), fd_(-1) {}
    ~UDSClient() { Close(); }

    bool Connect() {
        if (fd_ != -1) return true;

        fd_ = socket(AF_UNIX, SOCK_DGRAM, 0); // Use UDP for low latency
        if (fd_ < 0) return false;

        struct sockaddr_un addr;
        memset(&addr, 0, sizeof(addr));
        addr.sun_family = AF_UNIX;
        strncpy(addr.sun_path, path_.c_str(), sizeof(addr.sun_path) - 1);

        if (connect(fd_, (struct sockaddr*)&addr, sizeof(addr)) < 0) {
            Close();
            return false;
        }
        return true;
    }

    void Close() {
        if (fd_ != -1) {
            close(fd_);
            fd_ = -1;
        }
    }

    bool Send(const MetricEvent& event) {
        if (fd_ == -1 && !Connect()) return false;
        
        ssize_t sent = send(fd_, &event, sizeof(event), MSG_DONTWAIT);
        return sent == sizeof(event);
    }

private:
    std::string path_;
    int fd_;
};

} // namespace compute_sentry

#endif // COMPUTE_SENTRY_SPY_UDS_H
