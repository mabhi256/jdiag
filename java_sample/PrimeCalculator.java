// package java_sample;

import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.atomic.AtomicLong;

/**
 * CPU-intensive program that continuously calculates prime numbers.
 * Use it for CPU profiling.
 */
public class PrimeCalculator {
    private static final AtomicLong primesFound = new AtomicLong(0);
    private static final List<Long> largePrimes = new ArrayList<>();
    
    public static void main(String[] args) {
        System.out.println("Starting Prime Calculator - CPU intensive program");
        System.out.println("Monitor this with VisualVM to see CPU usage patterns");
        
        long startNumber = 1000000; // Start from 1 million
        long currentNumber = startNumber;
        
        while (true) {
            if (isPrime(currentNumber)) {
                primesFound.incrementAndGet();
                synchronized (largePrimes) {
                    largePrimes.add(currentNumber);
                    // Keep only last 1000 primes to prevent unlimited memory growth
                    if (largePrimes.size() > 1000) {
                        largePrimes.remove(0);
                    }
                }
                
                if (primesFound.get() % 100 == 0) {
                    System.out.printf("Found %d primes. Latest: %d%n", 
                        primesFound.get(), currentNumber);
                }
            }
            currentNumber++;
            
            // Add small delay every 10000 iterations to prevent 100% CPU usage
            if (currentNumber % 10000 == 0) {
                try {
                    Thread.sleep(10);
                } catch (InterruptedException e) {
                    Thread.currentThread().interrupt();
                    break;
                }
            }
        }
    }
    
    private static boolean isPrime(long n) {
        if (n < 2) return false;
        if (n == 2) return true;
        if (n % 2 == 0) return false;
        
        // Check odd divisors up to sqrt(n)
        for (long i = 3; i * i <= n; i += 2) {
            if (n % i == 0) {
                return false;
            }
        }
        return true;
    }
}