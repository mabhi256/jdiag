
import com.sun.tools.attach.*;
import java.io.File;
import java.util.*;
import javax.management.*;
import javax.management.openmbean.CompositeData;
import javax.management.openmbean.TabularData;
import javax.management.remote.*;

public class JMXClient {

    public static void main(String[] args) throws Exception {
        if (args.length < 3) {
            System.err.println("Usage: JMXClient <mode> <objectName|pattern> <pid|url> [attributes...]");
            System.exit(1);
        }

        String mode = args[0];
        String objectNameStr = args[1];
        String connection = args[2];
        String[] attributes = Arrays.copyOfRange(args, 3, args.length);

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
                resMap.put(attr, convertValue(value));
            } catch (Exception e) {
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

    private static Object convertValue(Object value) {
        if (value instanceof CompositeData cd) {
            Map<String, Object> map = new HashMap<>();
            for (String key : cd.getCompositeType().keySet()) {
                map.put(key, convertValue(cd.get(key))); // Recursively convert nested values
            }
            return map;
        }

        if (value instanceof TabularData td) {
            Map<String, Object> map = new HashMap<>();
            for (Object keyObj : td.keySet()) {
                CompositeData cd = td.get((Object[]) keyObj);
                String key = cd.get("key").toString();
                map.put(key, convertValue(cd.get("value")));
            }
            return map;
        }

        if (value != null && value.getClass().isArray()) {
            Object[] array = (Object[]) value;
            List<Object> list = new ArrayList<>();
            for (Object item : array) {
                list.add(convertValue(item));
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
