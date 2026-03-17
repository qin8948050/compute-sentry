cmake_minimum_required(VERSION 3.10)
project(compute-sentry-spy)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_FLAGS "-O3 -Wall -fPIC")

# Low overhead is critical for the spy library
add_definitions(-D_GNU_SOURCE)

include_directories(include)

add_library(compute-sentry-spy SHARED src/spy.cpp)

target_link_libraries(compute-sentry-spy dl)

# Specific flags for the spy library to minimize interference
set_target_properties(compute-sentry-spy PROPERTIES
    LIBRARY_OUTPUT_DIRECTORY ${CMAKE_BINARY_DIR}/lib
    INTERPROCEDURAL_OPTIMIZATION TRUE
)
