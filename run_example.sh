# #!/bin/bash

# # Remove previous logs to avoid mixing with old runs
# rm -f error_log.txt

# total_error=0
# total_mse=0
# total_mae=0
# num_runs=100  # Number of iterations

# # Start the timer
# start_time=$(date +%s.%N)

# for run in $(seq 1 $num_runs)
# do
#     echo "Starting run $run..."

#     # Create a temporary log file
#     temp_output=$(mktemp)

#     # Run all 3 parties in parallel, suppress logs except errors
#     for i in {0..2}
#     do
#       PID=$i go run sfgwas.go >> "$temp_output" 2>&1 &
#     done

#     wait  # Ensure all processes finish

#     # Extract error metrics from PID=1 output
#     avg_error=$(grep "Average Relative Error" "$temp_output" | awk '{print $NF}' | tail -n1)
#     mse=$(grep "Mean Squared Error" "$temp_output" | awk '{print $NF}' | tail -n1)
#     mae=$(grep "Mean Absolute Error" "$temp_output" | awk '{print $NF}' | tail -n1)

#     # Remove temp file after reading its contents
#     rm -f "$temp_output"

#     # Check if values are empty or invalid
#     if [[ -z "$avg_error" || ! "$avg_error" =~ ^[0-9.]+$ ]]; then
#         echo "Run $run: Error = (skipped)"
#         continue
#     fi
#     if [[ -z "$mse" || ! "$mse" =~ ^[0-9.]+$ ]]; then
#         echo "Run $run: MSE = (skipped)"
#         continue
#     fi
#     if [[ -z "$mae" || ! "$mae" =~ ^[0-9.]+$ ]]; then
#         echo "Run $run: MAE = (skipped)"
#         continue
#     fi

#     # Accumulate errors
#     total_error=$(echo "$total_error + $avg_error" | bc)
#     total_mse=$(echo "$total_mse + $mse" | bc)
#     total_mae=$(echo "$total_mae + $mae" | bc)

#     # Print only the error values
#     echo "Run $run: ARE = $avg_error, MSE = $mse, MAE = $mae"
# done

# # Stop the timer
# end_time=$(date +%s.%N)

# # Compute total execution time
# execution_time=$(echo "$end_time - $start_time" | bc)

# # Compute final average error metrics
# average_error=$(echo "scale=12; $total_error / $num_runs" | bc)
# average_mse=$(echo "scale=12; $total_mse / $num_runs" | bc)
# average_mae=$(echo "scale=12; $total_mae / $num_runs" | bc)

# echo "====================================="
# echo "Total Execution Time for $num_runs runs: ${execution_time}s"
# echo "Average Relative Error (ARE) over $num_runs runs: ${average_error}"
# echo "Mean Squared Error (MSE) over $num_runs runs: ${average_mse}"
# echo "Mean Absolute Error (MAE) over $num_runs runs: ${average_mae}"
# echo "====================================="

#!/bin/bash
for i in {0..2}; do
    PID=$i go run sfgwas.go &
done
wait