// package java_sample;

import java.util.Random;
import java.util.concurrent.*;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.concurrent.locks.ReentrantLock;

/**
 * Multithreaded program demonstrating various threading patterns.
 * Use it for monitoring thread activity, synchronization, and deadlock detection
 */
public class MultithreadSimulator {
    private static final int THREAD_COUNT = 8;
    private static final ExecutorService executor = Executors.newFixedThreadPool(THREAD_COUNT);
    private static final ScheduledExecutorService scheduler = Executors.newScheduledThreadPool(2);
    
    // Shared resources for synchronization demonstration
    private static final Object lock1 = new Object();
    private static final Object lock2 = new Object();
    private static final ReentrantLock reentrantLock = new ReentrantLock();
    private static final BlockingQueue<String> queue = new LinkedBlockingQueue<>();
    
    // Counters for monitoring
    private static final AtomicInteger tasksCompleted = new AtomicInteger(0);
    private static final AtomicInteger activeWorkers = new AtomicInteger(0);
    
    public static void main(String[] args) {
        System.out.println("Starting Multithreaded Simulator");
        System.out.println("Monitor thread activity and synchronization in VisualVM");
        System.out.println("Threads panel will show various thread states");
        
        // Start producer threads
        startProducers();
        
        // Start consumer threads
        startConsumers();
        
        // Start CPU-intensive workers
        startWorkers();
        
        // Start monitoring thread
        startMonitor();
        
        // Occasionally create deadlock scenario (commented out by default)
        // startDeadlockDemo();
        
        // Keep main thread alive
        try {
            Thread.currentThread().join();
        } catch (InterruptedException e) {
            shutdown();
        }
    }
    
    private static void startProducers() {
        for (int i = 0; i < 2; i++) {
            final int producerId = i;
            executor.submit(() -> {
                Thread.currentThread().setName("Producer-" + producerId);
                Random rand = new Random();
                
                while (!Thread.currentThread().isInterrupted()) {
                    try {
                        String item = "Item-" + System.currentTimeMillis() + "-" + producerId;
                        queue.put(item);
                        System.out.println("Producer " + producerId + " produced: " + item);
                        
                        Thread.sleep(rand.nextInt(1000) + 500); // 0.5-1.5 seconds
                    } catch (InterruptedException e) {
                        Thread.currentThread().interrupt();
                        break;
                    }
                }
            });
        }
    }
    
    private static void startConsumers() {
        for (int i = 0; i < 3; i++) {
            final int consumerId = i;
            executor.submit(() -> {
                Thread.currentThread().setName("Consumer-" + consumerId);
                Random rand = new Random();
                
                while (!Thread.currentThread().isInterrupted()) {
                    try {
                        String item = queue.take();
                        System.out.println("Consumer " + consumerId + " consumed: " + item);
                        
                        // Simulate processing time
                        Thread.sleep(rand.nextInt(800) + 200); // 0.2-1 second
                        
                        tasksCompleted.incrementAndGet();
                    } catch (InterruptedException e) {
                        Thread.currentThread().interrupt();
                        break;
                    }
                }
            });
        }
    }
    
    private static void startWorkers() {
        for (int i = 0; i < 3; i++) {
            final int workerId = i;
            executor.submit(() -> {
                Thread.currentThread().setName("Worker-" + workerId);
                Random rand = new Random();
                
                while (!Thread.currentThread().isInterrupted()) {
                    activeWorkers.incrementAndGet();
                    
                    try {
                        // Simulate different types of work with locks
                        int workType = rand.nextInt(3);
                        
                        switch (workType) {
                            case 0:
                                doSynchronizedWork(workerId);
                                break;
                            case 1:
                                doReentrantLockWork(workerId);
                                break;
                            case 2:
                                doCPUIntensiveWork(workerId);
                                break;
                        }
                        
                        Thread.sleep(rand.nextInt(2000) + 1000); // 1-3 seconds
                    } catch (InterruptedException e) {
                        Thread.currentThread().interrupt();
                        break;
                    } finally {
                        activeWorkers.decrementAndGet();
                    }
                }
            });
        }
    }
    
    private static void doSynchronizedWork(int workerId) throws InterruptedException {
        synchronized (lock1) {
            System.out.println("Worker " + workerId + " acquired lock1");
            Thread.sleep(100);
            
            // Nested synchronization
            synchronized (lock2) {
                System.out.println("Worker " + workerId + " acquired lock2");
                Thread.sleep(200);
            }
        }
    }
    
    private static void doReentrantLockWork(int workerId) throws InterruptedException {
        if (reentrantLock.tryLock(500, TimeUnit.MILLISECONDS)) {
            try {
                System.out.println("Worker " + workerId + " acquired reentrant lock");
                Thread.sleep(300);
            } finally {
                reentrantLock.unlock();
            }
        } else {
            System.out.println("Worker " + workerId + " failed to acquire reentrant lock");
        }
    }
    
    private static void doCPUIntensiveWork(int workerId) {
        System.out.println("Worker " + workerId + " doing CPU intensive work");
        
        // Calculate some primes to use CPU
        int count = 0;
        for (int i = 2; i < 10000 && !Thread.currentThread().isInterrupted(); i++) {
            boolean isPrime = true;
            for (int j = 2; j * j <= i; j++) {
                if (i % j == 0) {
                    isPrime = false;
                    break;
                }
            }
            if (isPrime) count++;
        }
        
        System.out.println("Worker " + workerId + " found " + count + " primes");
    }
    
    private static void startMonitor() {
        scheduler.scheduleAtFixedRate(() -> {
            Thread.currentThread().setName("Monitor");
            System.out.printf("=== Status: Tasks completed: %d, Active workers: %d, Queue size: %d ===%n",
                tasksCompleted.get(), activeWorkers.get(), queue.size());
        }, 5, 5, TimeUnit.SECONDS);
    }
    
    // Uncomment to demonstrate deadlock detection in VisualVM
    /*
    private static void startDeadlockDemo() {
        scheduler.scheduleAtFixedRate(() -> {
            executor.submit(() -> {
                Thread.currentThread().setName("DeadlockThread1");
                synchronized (lock1) {
                    try { Thread.sleep(100); } catch (InterruptedException e) {}
                    synchronized (lock2) {
                        System.out.println("Thread1 got both locks");
                    }
                }
            });
            
            executor.submit(() -> {
                Thread.currentThread().setName("DeadlockThread2");
                synchronized (lock2) {
                    try { Thread.sleep(100); } catch (InterruptedException e) {}
                    synchronized (lock1) {
                        System.out.println("Thread2 got both locks");
                    }
                }
            });
        }, 30, 30, TimeUnit.SECONDS);
    }
    */
    
    private static void shutdown() {
        System.out.println("Shutting down...");
        executor.shutdown();
        scheduler.shutdown();
        try {
            if (!executor.awaitTermination(5, TimeUnit.SECONDS)) {
                executor.shutdownNow();
            }
            if (!scheduler.awaitTermination(5, TimeUnit.SECONDS)) {
                scheduler.shutdownNow();
            }
        } catch (InterruptedException e) {
            executor.shutdownNow();
            scheduler.shutdownNow();
        }
    }
}