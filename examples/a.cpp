  #include <iostream>
#include <thread>
#include <unistd.h>
void BackgroundSender() {                                                                                                                                                                                        
      std::cout << "线程开始\n";
      sleep(5);
      std::cout << "线程结束\n";
  }

  int main() {
      std::cout << "1. 主线程开始\n";

      std::thread t(BackgroundSender);  // 这里立即返回，不阻塞！

      std::cout << "2. 主线程继续执行\n";  // 立即打印，不等 BackgroundSender

      t.join();  // 这里才会等待线程结束
      std::cout << "3. 主线程结束\n";
  }
