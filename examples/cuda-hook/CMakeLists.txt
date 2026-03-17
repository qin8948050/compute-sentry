cmake_minimum_required(VERSION 3.10)
project(cuda-hook-example)

set(CMAKE_CXX_STANDARD 11)

# 编译测试程序
add_executable(test_cuda test_cuda.cpp)
target_link_libraries(test_cuda cuda)

# 编译 hook 库
add_library(hook SHARED cuda_hook.cpp)
target_link_libraries(hook dl)
set_target_properties(hook PROPERTIES PREFIX "")

# 安装
install(TARGETS test_cuda RUNTIME DESTINATION bin)
install(TARGETS hook LIBRARY DESTINATION lib)
