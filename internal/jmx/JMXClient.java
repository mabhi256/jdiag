import com.sun.tools.attach.*;
import java.io.*;
import java.time.LocalDateTime;
import java.time.format.DateTimeFormatter;
import java.util.*;
import javax.management.*;
import javax.management.openmbean.CompositeData;
import javax.management.openmbean.TabularData;
import javax.management.remote.*;

public class JMXClient {
    private static PrintWriter logWriter;
    private static final boolean LOG_ENABLED = false;
    private static final String LOG_FILE = "jmx_debug_java.log";

    public static void main(String[] args) throws Exception {
        if (LOG_ENABLED) {
            initializeLogging();
        }

        if (args.length < 3) {
            System.err.println("Usage: JMXClient <mode> <objectName|pattern> <pid|url> [attributes...]");
            logError("Insufficient arguments provided");
            System.exit(1);
        }

        String mode = args[0];
        String objectNameStr = args[1];
        String connection = args[2];
        String[] attributes = Arrays.copyOfRange(args, 3, args.length);

        try {
            MBeanServerConnection mbsc = connect(connection);

            if (mode.equals("single")) {
                ObjectName objectName = new ObjectName(objectNameStr);
                Map<String, Object> result = queryMbean(mbsc, objectName, attributes);
                printJSON(result);
            } else if (mode.equals("pattern")) {
                ObjectName objectName = new ObjectName(objectNameStr);
                List<Map<String, Object>> result = queryMbeanPattern(mbsc, objectName, attributes);
                printJSON(result);
            }
        } catch (Exception e) {
            logError("Main execution failed: " + e.getMessage());
            throw e;
        } finally {
            closeLogging();
        }
    }

    private static void initializeLogging() {
        try {
            logWriter = new PrintWriter(new FileWriter(LOG_FILE, true));
            log("=== JMXClient session started at " + LocalDateTime.now().format(DateTimeFormatter.ISO_LOCAL_DATE_TIME)
                    + " ===");
        } catch (IOException e) {
            System.err.println("Failed to initialize logging: " + e.getMessage());
        }
    }

    private static void log(String message) {
        if (logWriter != null) {
            logWriter.println("[" + LocalDateTime.now().format(DateTimeFormatter.ISO_LOCAL_TIME) + "] " + message);
            logWriter.flush();
        }
    }

    private static void logError(String message) {
        log("ERROR: " + message);
        System.err.println("ERROR: " + message);
    }

    private static void closeLogging() {
        if (logWriter != null) {
            log("=== JMXClient session ended ===\n");
            logWriter.close();
        }
    }

    private static MBeanServerConnection connect(String connection) throws Exception {
        if (connection.matches("\\d+")) {
            connection = getAddrFromPID(connection);
        }

        JMXServiceURL serviceURL = new JMXServiceURL(connection);
        JMXConnector connector = JMXConnectorFactory.connect(serviceURL);
        return connector.getMBeanServerConnection();
    }

    private static String getAddrFromPID(String pidStr) throws Exception {
        String localConnectorProperty = "com.sun.management.jmxremote.localConnectorAddress";
        VirtualMachine vm = VirtualMachine.attach(pidStr);

        try {
            String addr = vm
                    .getAgentProperties()
                    .getProperty(localConnectorProperty);

            if (addr == null) {
                // Try to start the management agent
                // First try Java 9+ path
                try {
                    vm.startManagementAgent(null);
                    addr = vm.getAgentProperties().getProperty(localConnectorProperty);
                } catch (Exception e) {
                    // Fallback for older Java versions (Java 8 and below)
                    String javaHome = vm.getSystemProperties().getProperty("java.home");
                    String agentPath = javaHome + File.separator + "lib" + File.separator + "management-agent.jar";

                    // Check if the agent file exists
                    File agentFile = new File(agentPath);
                    if (!agentFile.exists()) {
                        // Try alternative location (some JDK distributions)
                        agentPath = javaHome + File.separator + "jre" + File.separator + "lib" + File.separator
                                + "management-agent.jar";
                        agentFile = new File(agentPath);
                    }

                    if (agentFile.exists()) {
                        vm.loadAgent(agentPath);
                        addr = vm.getAgentProperties().getProperty(localConnectorProperty);
                    } else {
                        throw new RuntimeException("Management agent not found. Tried: " + agentPath);
                    }
                }
            }

            if (addr == null) {
                throw new RuntimeException("Failed to get JMX connector address after loading agent");
            }

            return addr;
        } finally {
            vm.detach();
        }
    }

    private static Map<String, Object> queryMbean(
            MBeanServerConnection mbsc,
            ObjectName objectName,
            String[] attributes) throws Exception {
        Map<String, Object> resMap = new HashMap<>();

        // If no specific attributes requested, get all available
        if (attributes.length == 0) {
            MBeanInfo info = mbsc.getMBeanInfo(objectName);
            attributes = Arrays.stream(info.getAttributes())
                    .map(MBeanAttributeInfo::getName)
                    .toArray(String[]::new);
        }

        // Get each attribute value
        for (String attr : attributes) {
            try {
                Object value = mbsc.getAttribute(objectName, attr);
                Object convertedValue = convertValue(value, attr, 0);
                resMap.put(attr, convertedValue);
                log(attr + " = " + convertedValue);
            } catch (Exception e) {
                logError("Failed to get attribute '" + attr + "' from " + objectName + ": " + e.getMessage());
                resMap.put(attr, null);
            }
        }

        return resMap;
    }

    private static List<Map<String, Object>> queryMbeanPattern(
            MBeanServerConnection mbsc,
            ObjectName pattern,
            String[] attributes) throws Exception {
        List<Map<String, Object>> resMap = new ArrayList<>();
        Set<ObjectName> objectNames = mbsc.queryNames(pattern, null);

        for (ObjectName objectName : objectNames) {
            Map<String, Object> data = queryMbean(mbsc, objectName, attributes);
            data.put("objectName", objectName.toString());
            resMap.add(data);
        }

        return resMap;
    }

    private static Object[] convertToKeyArray(Object keyObj) {
        if (keyObj instanceof Object[] objects) {
            return objects;
        } else if (keyObj instanceof List<?> list) {
            return list.toArray();
        } else {
            // Single key, wrap in array
            return new Object[] { keyObj };
        }
    }

    private static Object convertValue(Object value, String context, int depth) {
        if (value == null) {
            return null;
        }

        if (value instanceof CompositeData cd) {
            Map<String, Object> map = new HashMap<>();

            try {
                Set<String> keys = cd.getCompositeType().keySet();

                for (String key : keys) {
                    try {
                        Object nestedValue = cd.get(key);
                        Object converted = convertValue(nestedValue, context + "." + key, depth + 1);
                        map.put(key, converted);
                    } catch (Exception e) {
                        logError("Failed to convert CompositeData key '" + key + "': " + e.getMessage());
                        map.put(key, null);
                    }
                }

                return map;
            } catch (Exception e) {
                logError("Failed to process CompositeData: " + e.getMessage());
                return null;
            }
        }

        if (value instanceof TabularData td) {
            Map<String, Object> map = new HashMap<>();

            try {
                // List<String> indexNames = td.getTabularType().getIndexNames();

                for (Object keyObj : td.keySet()) {
                    try {
                        Object[] keyArray = convertToKeyArray(keyObj);

                        CompositeData rowData = td.get(keyArray);

                        if (rowData == null) {
                            continue;
                        }

                        // Create a meaningful key from the index values
                        String rowKey;
                        if (keyArray.length == 1 && keyArray[0] != null) {
                            rowKey = keyArray[0].toString();
                        } else if (keyArray.length > 1) {
                            rowKey = String.join(".", Arrays.stream(keyArray)
                                    .map(k -> k != null ? k.toString() : "null")
                                    .toArray(String[]::new));
                        } else {
                            rowKey = "row_" + map.size();
                        }

                        Object convertedValue = convertValue(rowData, context + "[" + rowKey + "]", depth + 1);
                        map.put(rowKey, convertedValue);

                    } catch (Exception e) {
                        logError("Failed to convert TabularData entry: " + e.getMessage());
                    }
                }

                return map;
            } catch (Exception e) {
                logError("Failed to process TabularData: " + e.getMessage());
                return null;
            }
        }

        // Handle arrays
        if (value.getClass().isArray()) {
            Object[] array = (Object[]) value;
            List<Object> list = new ArrayList<>();

            for (int i = 0; i < array.length; i++) {
                Object convertedItem = convertValue(array[i], context + "[" + i + "]", depth + 1);
                list.add(convertedItem);
            }

            return list;
        }

        // For primitive types and strings, return as-is
        return value;
    }

    private static void printJSON(Object obj) {
        if (obj == null) {
            System.out.print("null");
        } else if (obj instanceof String str) {
            printJSONString(str);
        } else if (obj instanceof Number || obj instanceof Boolean) {
            System.out.print(obj);
        } else if (obj instanceof Map map) {
            printJSONObject(map);
        } else if (obj instanceof List) {
            printJSONArray((List<?>) obj);
        } else {
            printJSONString(obj.toString());
        }
    }

    private static void printJSONString(String str) {
        System.out.print('"');

        System.out.print(str.replace("\\", "\\\\")
                .replace("\"", "\\\"")
                .replace("\n", "\\n")
                .replace("\r", "\\r")
                .replace("\t", "\\t"));

        System.out.print('"');
    }

    private static void printJSONObject(Map<?, ?> map) {
        System.out.print('{');

        boolean first = true;
        for (Map.Entry<?, ?> entry : map.entrySet()) {
            if (!first) {
                System.out.print(',');
            }
            printJSONString(entry.getKey().toString());
            System.out.print(':');
            printJSON(entry.getValue());
            first = false;
        }

        System.out.print('}');
    }

    private static void printJSONArray(List<?> list) {
        System.out.print("[");

        for (int i = 0; i < list.size(); i++) {
            if (i > 0) {
                System.out.print(",");
            }
            printJSON(list.get(i));
        }

        System.out.print("]");
    }
}