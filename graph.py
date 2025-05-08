import numpy as np
import matplotlib.pyplot as plt

# Load data from results.txt
data = np.loadtxt("results.txt")

x = data[:, 0]  # x values
computed_y = data[:, 1]  # Computed sin(x)
expected_y = data[:, 2]  # True sin(x)

# Approximate function
approximate_y = 2.33 * np.sin((np.pi / 9) * x)

# Plot the results
plt.figure(figsize=(10, 5))
plt.plot(x, expected_y, label="Expected sin(x)", color="blue", linestyle="dashed")
plt.plot(x, computed_y, label="Computed (Remez Approximation)", color="red")
plt.plot(x, approximate_y, label="Approximate Function (2.33 * sin(pi/9 * x))", color="green", linestyle="dotted")

# Labels and legend
plt.xlabel("x (radians)")
plt.ylabel("sin(x)")
plt.title("Comparison of Expected sin(x), Remez Approximation, and Approximate Function")
plt.legend()
plt.grid(True)

# Show the plot
plt.show()
