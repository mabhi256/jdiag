// package java_sample;

import java.time.LocalDateTime;
import java.time.format.DateTimeFormatter;
import java.util.*;
import java.util.concurrent.*;
import java.util.concurrent.atomic.AtomicLong;

/**
 * Simulates a web service with database operations, caching, and request processing.
 * Demonstrates realistic application patterns for:
 * - Request processing with varying load
 * - Memory caching with expiration
 * - Database connection pooling simulation
 * - Periodic background tasks
 */
public class WebServiceSimulator {
    
    // Configuration
    private static final int MAX_CONCURRENT_REQUESTS = 20;
    private static final int CACHE_SIZE = 1000;
    private static final int DB_POOL_SIZE = 10;
    
    // Core components
    private static final ExecutorService requestProcessor = Executors.newFixedThreadPool(MAX_CONCURRENT_REQUESTS);
    private static final ScheduledExecutorService scheduler = Executors.newScheduledThreadPool(4);
    private static final RequestCache cache = new RequestCache(CACHE_SIZE);
    private static final DatabasePool dbPool = new DatabasePool(DB_POOL_SIZE);
    
    // Metrics
    private static final AtomicLong requestCounter = new AtomicLong(0);
    private static final AtomicLong cacheHits = new AtomicLong(0);
    private static final AtomicLong cacheMisses = new AtomicLong(0);
    private static final AtomicLong dbQueries = new AtomicLong(0);
    
    public static void main(String[] args) {
        System.out.println("Starting Web Service Simulator");
        System.out.println("This simulates a realistic web application with:");
        System.out.println("- HTTP request processing");
        System.out.println("- Memory caching with LRU eviction");
        System.out.println("- Database connection pooling");
        System.out.println("- Background maintenance tasks");
        System.out.println("Perfect for comprehensive VisualVM monitoring!");
        System.out.println();
        
        // Start request generators with different patterns
        startLightTrafficGenerator();
        startBurstTrafficGenerator();
        startHeavyUserSimulator();
        
        // Start background maintenance tasks
        startCacheCleanup();
        startMetricsReporting();
        startDatabaseMaintenance();
        
        // Keep main thread alive
        try {
            Thread.currentThread().join();
        } catch (InterruptedException e) {
            shutdown();
        }
    }
    
    private static void startLightTrafficGenerator() {
        scheduler.scheduleAtFixedRate(() -> {
            for (int i = 0; i < ThreadLocalRandom.current().nextInt(1, 4); i++) {
                processRequest(new UserRequest("light-user-" + i, RequestType.READ));
            }
        }, 1, 2, TimeUnit.SECONDS);
    }
    
    private static void startBurstTrafficGenerator() {
        scheduler.scheduleAtFixedRate(() -> {
            // Generate burst of requests
            int burstSize = ThreadLocalRandom.current().nextInt(5, 15);
            System.out.println("Generating burst traffic: " + burstSize + " requests");
            
            for (int i = 0; i < burstSize; i++) {
                RequestType type = ThreadLocalRandom.current().nextBoolean() ? RequestType.READ : RequestType.WRITE;
                processRequest(new UserRequest("burst-user-" + i, type));
            }
        }, 10, 15, TimeUnit.SECONDS);
    }
    
    private static void startHeavyUserSimulator() {
        scheduler.scheduleAtFixedRate(() -> {
            // Simulate heavy user doing complex operations
            processRequest(new UserRequest("heavy-user", RequestType.COMPLEX_QUERY));
        }, 3, 5, TimeUnit.SECONDS);
    }
    
    private static void processRequest(UserRequest request) {
        requestProcessor.submit(() -> {
            Thread.currentThread().setName("RequestProcessor-" + requestCounter.incrementAndGet());
            
            try {
                handleRequest(request);
            } catch (Exception e) {
                System.err.println("Error processing request: " + e.getMessage());
            }
        });
    }
    
    private static void handleRequest(UserRequest request) throws InterruptedException {
        String userId = request.getUserId();
        RequestType type = request.getType();
        
        System.out.printf("[%s] Processing %s request from %s%n", 
            LocalDateTime.now().format(DateTimeFormatter.ofPattern("HH:mm:ss")), 
            type, userId);
        
        switch (type) {
            case READ:
                handleReadRequest(userId);
                break;
            case WRITE:
                handleWriteRequest(userId);
                break;
            case COMPLEX_QUERY:
                handleComplexQuery(userId);
                break;
        }
    }
    
    private static void handleReadRequest(String userId) throws InterruptedException {
        // Try cache first
        String cacheKey = "user_data_" + userId;
        UserData data = cache.get(cacheKey);
        
        if (data != null) {
            cacheHits.incrementAndGet();
            System.out.println("Cache hit for " + userId);
            Thread.sleep(ThreadLocalRandom.current().nextInt(10, 50)); // Fast response
        } else {
            cacheMisses.incrementAndGet();
            System.out.println("Cache miss for " + userId + ", querying database");
            
            // Simulate database query
            data = queryDatabase(userId);
            cache.put(cacheKey, data);
            Thread.sleep(ThreadLocalRandom.current().nextInt(100, 300)); // Slower response
        }
    }
    
    private static void handleWriteRequest(String userId) throws InterruptedException {
        System.out.println("Writing data for " + userId);
        
        // Simulate database write
        DatabaseConnection conn = dbPool.getConnection();
        try {
            Thread.sleep(ThreadLocalRandom.current().nextInt(150, 400)); // DB write time
            dbQueries.incrementAndGet();
            
            // Invalidate cache
            cache.invalidate("user_data_" + userId);
            
        } finally {
            dbPool.releaseConnection(conn);
        }
    }
    
    private static void handleComplexQuery(String userId) throws InterruptedException {
        System.out.println("Processing complex query for " + userId);
        
        DatabaseConnection conn = dbPool.getConnection();
        try {
            // Simulate complex join queries
            for (int i = 0; i < 3; i++) {
                Thread.sleep(ThreadLocalRandom.current().nextInt(200, 500));
                dbQueries.incrementAndGet();
            }
            
            // Simulate data processing
            List<String> results = new ArrayList<>();
            for (int i = 0; i < 1000; i++) {
                results.add("Result " + i + " for " + userId + " with timestamp " + System.currentTimeMillis());
            }
            
            // Simulate result serialization
            StringBuilder json = new StringBuilder();
            json.append("{\"results\":[");
            for (String result : results) {
                json.append("\"").append(result).append("\",");
            }
            json.append("]}");
            
        } finally {
            dbPool.releaseConnection(conn);
        }
    }
    
    private static UserData queryDatabase(String userId) throws InterruptedException {
        DatabaseConnection conn = dbPool.getConnection();
        try {
            Thread.sleep(ThreadLocalRandom.current().nextInt(50, 200)); // DB query time
            dbQueries.incrementAndGet();
            
            return new UserData(userId, "User data for " + userId, System.currentTimeMillis());
        } finally {
            dbPool.releaseConnection(conn);
        }
    }
    
    private static void startCacheCleanup() {
        scheduler.scheduleAtFixedRate(() -> {
            Thread.currentThread().setName("CacheCleanup");
            int cleaned = cache.cleanup();
            if (cleaned > 0) {
                System.out.println("Cache cleanup: removed " + cleaned + " expired entries");
            }
        }, 30, 30, TimeUnit.SECONDS);
    }
    
    private static void startMetricsReporting() {
        scheduler.scheduleAtFixedRate(() -> {
            Thread.currentThread().setName("MetricsReporter");
            System.out.printf("=== METRICS === Requests: %d, Cache hits: %d, Cache misses: %d, DB queries: %d, Cache size: %d%n",
                requestCounter.get(), cacheHits.get(), cacheMisses.get(), dbQueries.get(), cache.size());
        }, 10, 10, TimeUnit.SECONDS);
    }
    
    private static void startDatabaseMaintenance() {
        scheduler.scheduleAtFixedRate(() -> {
            Thread.currentThread().setName("DBMaintenance");
            System.out.println("Running database maintenance...");
            
            // Simulate maintenance work
            try {
                Thread.sleep(ThreadLocalRandom.current().nextInt(1000, 3000));
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
            }
            
            System.out.println("Database maintenance completed");
        }, 60, 120, TimeUnit.SECONDS);
    }
    
    private static void shutdown() {
        System.out.println("Shutting down web service...");
        requestProcessor.shutdown();
        scheduler.shutdown();
        // In a real application, we'd also close database connections, etc.
    }
    
    // Supporting classes
    static class UserRequest {
        private final String userId;
        private final RequestType type;
        
        UserRequest(String userId, RequestType type) {
            this.userId = userId;
            this.type = type;
        }
        
        String getUserId() { return userId; }
        RequestType getType() { return type; }
    }
    
    enum RequestType {
        READ, WRITE, COMPLEX_QUERY
    }
    
    static class UserData {
        private final String userId;
        private final String data;
        private final long timestamp;
        
        UserData(String userId, String data, long timestamp) {
            this.userId = userId;
            this.data = data;
            this.timestamp = timestamp;
        }
        
        boolean isExpired(long ttl) {
            return System.currentTimeMillis() - timestamp > ttl;
        }
    }
    
    static class RequestCache {
        private final Map<String, UserData> cache = new ConcurrentHashMap<>();
        private final int maxSize;
        private final long ttl = 60000; // 1 minute TTL
        
        RequestCache(int maxSize) {
            this.maxSize = maxSize;
        }
        
        UserData get(String key) {
            UserData data = cache.get(key);
            if (data != null && data.isExpired(ttl)) {
                cache.remove(key);
                return null;
            }
            return data;
        }
        
        void put(String key, UserData data) {
            if (cache.size() >= maxSize) {
                // Simple LRU: remove oldest entry
                String oldestKey = cache.keySet().iterator().next();
                cache.remove(oldestKey);
            }
            cache.put(key, data);
        }
        
        void invalidate(String key) {
            cache.remove(key);
        }
        
        int cleanup() {
            int removed = 0;
            Iterator<Map.Entry<String, UserData>> it = cache.entrySet().iterator();
            while (it.hasNext()) {
                if (it.next().getValue().isExpired(ttl)) {
                    it.remove();
                    removed++;
                }
            }
            return removed;
        }
        
        int size() {
            return cache.size();
        }
    }
    
    static class DatabaseConnection {
        private final int id;
        DatabaseConnection(int id) { this.id = id; }
        int getId() { return id; }
    }
    
    static class DatabasePool {
        private final BlockingQueue<DatabaseConnection> pool;
        
        DatabasePool(int size) {
            pool = new LinkedBlockingQueue<>();
            for (int i = 0; i < size; i++) {
                pool.offer(new DatabaseConnection(i));
            }
        }
        
        DatabaseConnection getConnection() throws InterruptedException {
            return pool.take(); // Blocks if no connections available
        }
        
        void releaseConnection(DatabaseConnection conn) {
            pool.offer(conn);
        }
    }
}