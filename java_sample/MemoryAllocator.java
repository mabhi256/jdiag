package java_sample;

import java.util.*;
import java.util.concurrent.ThreadLocalRandom;

/**
 * Memory-intensive program that demonstrates various allocation patterns.
 * Use it for monitoring heap usage, GC activity, and memory leaks 
 */
public class MemoryAllocator {
    private static final List<Object> permanentStorage = new ArrayList<>();
    private static final Map<String, List<byte[]>> categoryStorage = new HashMap<>();
    
    public static void main(String[] args) {
        System.out.println("Starting Memory Allocator - Memory intensive program");
        System.out.println("Monitor heap usage and GC activity with VisualVM");
        System.out.println("Run with: java -Xmx512m -XX:+UseG1GC MemoryAllocator");
        
        // Initialize categories
        categoryStorage.put("small", new ArrayList<>());
        categoryStorage.put("medium", new ArrayList<>());
        categoryStorage.put("large", new ArrayList<>());
        
        int cycle = 0;
        while (true) {
            cycle++;
            System.out.printf("Memory allocation cycle %d%n", cycle);
            
            // Phase 1: Allocate lots of small objects (simulates high allocation rate)
            allocateSmallObjects(1000);
            sleep(500);
            
            // Phase 2: Allocate medium objects and keep some
            allocateMediumObjects(100);
            sleep(300);
            
            // Phase 3: Allocate large objects periodically
            if (cycle % 5 == 0) {
                allocateLargeObjects(10);
            }
            sleep(200);
            
            // Phase 4: Simulate memory pressure by keeping some objects
            simulateMemoryPressure();
            sleep(1000);
            
            // Phase 5: Cleanup some objects to trigger GC
            if (cycle % 10 == 0) {
                performCleanup();
                System.gc(); // Suggest garbage collection
            }
            
            sleep(2000);
        }
    }
    
    private static void allocateSmallObjects(int count) {
        List<String> temp = new ArrayList<>();
        for (int i = 0; i < count; i++) {
            // Create strings of varying sizes
            StringBuilder sb = new StringBuilder();
            int size = ThreadLocalRandom.current().nextInt(10, 100);
            for (int j = 0; j < size; j++) {
                sb.append((char) ('A' + (j % 26)));
            }
            temp.add(sb.toString());
            
            // Keep some objects alive longer
            if (i % 50 == 0) {
                permanentStorage.add(sb.toString());
            }
        }
        // temp goes out of scope, eligible for GC
    }
    
    private static void allocateMediumObjects(int count) {
        List<byte[]> medium = categoryStorage.get("medium");
        
        for (int i = 0; i < count; i++) {
            // Create arrays of 1KB to 10KB
            int size = ThreadLocalRandom.current().nextInt(1024, 10240);
            byte[] data = new byte[size];
            ThreadLocalRandom.current().nextBytes(data);
            
            medium.add(data);
            
            // Occasionally remove old objects
            if (medium.size() > 500) {
                medium.remove(0);
            }
        }
    }
    
    private static void allocateLargeObjects(int count) {
        List<byte[]> large = categoryStorage.get("large");
        
        for (int i = 0; i < count; i++) {
            // Create arrays of 100KB to 1MB
            int size = ThreadLocalRandom.current().nextInt(102400, 1048576);
            byte[] data = new byte[size];
            ThreadLocalRandom.current().nextBytes(data);
            
            large.add(data);
            
            // Remove old large objects to prevent OutOfMemoryError
            if (large.size() > 20) {
                large.remove(0);
            }
        }
    }
    
    private static void simulateMemoryPressure() {
        // Create temporary pressure on heap
        Map<Integer, String> tempMap = new HashMap<>();
        for (int i = 0; i < 1000; i++) {
            tempMap.put(i, "Temporary data item " + i + " with some extra content to use memory");
        }
        
        // Process the map to simulate work
        tempMap.values().forEach(String::length);
        
        // Map goes out of scope, eligible for GC
    }
    
    private static void performCleanup() {
        System.out.println("Performing cleanup...");
        
        // Clean permanent storage
        if (permanentStorage.size() > 10000) {
            permanentStorage.subList(0, 5000).clear();
        }
        
        // Clean category storage
        for (List<?> list : categoryStorage.values()) {
            if (list.size() > 100) {
                list.subList(0, list.size() / 2).clear();
            }
        }
        
        System.out.println("Cleanup completed");
    }
    
    private static void sleep(long millis) {
        try {
            Thread.sleep(millis);
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
            System.exit(1);
        }
    }
}