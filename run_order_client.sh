#!/bin/bash

# Number of order processes to start
num_orders=10

# Array to store the process IDs
pids=()

# Start 10 order processes in the background
for ((i = 1; i <= num_orders; i++)); do
    ./order &
    pids+=($!)  # Store the process ID in the array
done

# Wait for all order processes to complete
for pid in "${pids[@]}"; do
    wait $pid
done

echo "All processes have completed."