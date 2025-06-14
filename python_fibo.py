import time

def fibonacci_naive(n):
    if n <= 1:
        return n
    else:
        return fibonacci_naive(n-1) + fibonacci_naive(n-2)

start_time = time.time()
print(fibonacci_naive(35))
print("--- %s seconds ---" % (time.time() - start_time))
