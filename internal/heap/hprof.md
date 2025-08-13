# Java Profile 1.0.2 Format Specification

HPROF is a binary format used for Java heap dumps and profiling data.

## File Structure

Every HPROF file consists of:

1. **Header** - Format version and timestamp
2. **Records** - A sequence of tagged records containing the actual data

```bash
[Header]
[Record 1]
[Record 2]
...
[Record N]
```

When the HPROF format was originally designed (early 2000s), most computers were 32-bit systems:

- CPU registers were 32 bits wide
- Memory addresses were 32 bits
- "Natural" data size was 32 bits (4 bytes)

## Header Format

```bash
"JAVA PROFILE 1.0.2\0"      Null-terminated string
u4                          Size of IDs (usually pointer size)
u4                          High word of timestamp
u4                          Low word of timestamp (ms since 1/1/70)
```

- `u4` = 4-byte unsigned integer
- null-terminated aka. 0-terminated

```bash
J  A  V  A     P  R  O  F  I  L  E     1  .  0  .  2  \0
4A 41 56 41 20 50 52 4F 46 49 4C 45 20 31 2E 30 2E 32 00
```

### Timestamps are 64-bit

- Unix timestamps count milliseconds since January 1, 1970
- A 32-bit number can only go up to ~4.3 billion i.e. < 50 days
- Therefore hprof stores it as two 32-bit values:
  - *High word*: Upper 32 bits (most significant)
  - *Low word*: Lower 32 bits

## Record Format

Each record has a consistent structure:

```bash
u1      a TAG denoting record type
u4      Microseconds since header timestamp
u4      Bytes remaining in record (excluding tag + ts + length)
[body]  Record-specific data
```

## Record Types

### Basic Records

#### HPROF_UTF8

Defines UTF-8 encoded strings referenced by ID throughout the file.

```bash
id          ID for this string
[u1]*       UTF-8 characters (no null terminator)
```

#### HPROF_LOAD_CLASS

Records when a class is loaded into the JVM.

```bash
u4      Unique class serial number
id      Object ID of the Class object
u4      Stack trace serial number when loaded
id      class name ID - reference to UTF8 string
```

#### HPROF_UNLOAD_CLASS

Records when a class is unloaded.

```bash
u4      Serial number of unloaded class
```

### Stack Trace Records

#### HPROF_FRAME

Defines a single stack frame.

```bash
id      stack frame ID
id      Method name ID (UTF8 reference)
id      Method signature ID (UTF8 reference)
id      Source file name ID (UTF8 reference)
u4      Class serial number
i4      Line number. >0: normal line
                     -1: unknown
                     -2: compiled method
                     -3: native method
```

#### HPROF_TRACE

Defines a complete stack trace as a sequence of frames.

```bash
u4          Stack trace serial number
u4          Thread serial number that produced this trace
u4          Number of frames
[id]*       Array of stack frame IDs
```

### Thread Records

#### HPROF_START_THREAD

Records when a thread starts.

```bash
u4      Thread serial number
id      Thread object ID
u4      Stack trace serial number at creation
id      thread name ID (UTF8 reference)
id      thread group name ID
id      Parent thread group name ID
```

#### HPROF_END_THREAD

Records when a thread terminates.

```bash
u4      Thread serial number
```

### Heap Analysis Records

#### HPROF_ALLOC_SITES

Shows where objects are being allocated (post-GC analysis).

```bash
u2      flags   // 0x0001: incremental vs complete
                // 0x0002: sorted by allocation vs live
                // 0x0004: force GC
u4      Cutoff ratio
u4      Total live bytes
u4      Total live instances
u8      Total bytes  allocated
u8      Total instances  allocated
u4      Number of allocation sites

// For each site:
[u1     is array    // 0=normal object, 2=obj[], 4=bool[], 5=char[], 
                    // 6=float[], 7=double[], 8=byte[], 
                    // 9=short[], 10=int[], 11=long[]
u4      Class serial number (may be 0 during startup)
u4      Stack trace serial number
u4      Bytes currently alive
u4      Instances currently alive
u4      Total bytes allocated here
u4]     Total instances allocated here
```

#### HPROF_HEAP_SUMMARY

Overall heap statistics.

```bash
u4      Total live bytes
u4      Total live instances
u8      Total bytes ever allocated
u8      Total instances ever allocated
```

### Heap Dump Records

#### HPROF_HEAP_DUMP / HPROF_HEAP_DUMP_SEGMENT

Contains detailed heap information as sub-records. Segments allow large heaps to be split across multiple records.

```bash
[heap dump sub-records]*
```

## Heap Dump Sub-Records

There are four kinds of heap dump sub-records, each prefixed by a u1 type identifier:

```bash
u1                      sub_record_type       
```

### GC Root Records

These identify objects that are garbage collectionroots (won't be collected):

#### HPROF_GC_ROOT_UNKNOWN

```bash
id      Object ID of the root
```

#### HPROF_GC_ROOT_THREAD_OBJ

```bash
id      Thread object ID (may be 0 for newly attached JNI threads)
u4      Thread sequence number
u4      Stack trace sequence number
```

#### HPROF_GC_ROOT_JNI_GLOBAL

```bash
id      Object ID
id      JNI global reference ID
```

#### HPROF_GC_ROOT_JNI_LOCAL

```bash
id      Object ID
u4      Thread serial number
u4      Frame number in stack trace (-1 if empty)
```

#### HPROF_GC_ROOT_JAVA_FRAME

```bash
id      Object ID
u4      Thread serial number
u4      Frame number in stack trace (-1 if empty)
```

#### HPROF_GC_ROOT_NATIVE_STACK

```bash
id      Object ID
u4      Thread serial number
```

#### HPROF_GC_ROOT_STICKY_CLASS

```bash
id      Object ID
```

#### HPROF_GC_ROOT_THREAD_BLOCK

```bash
id      Object ID
u4      Thread serial number
```

#### HPROF_GC_ROOT_MONITOR_USED

```bash
id      Object ID
```

### Object Dump Records

#### HPROF_GC_CLASS_DUMP

```bash
id          Class object ID
u4          Allocation stack trace serial number
id          Super class object ID
id          Class loader object ID
id          Signers object ID
id          Protection domain object ID
id          Reserved field
id          Reserved field
u4          Size of instances in bytes

u2          Number of constant pool entries
[u2         Constant pool index
ty          Type (2=object, 4=boolean, 5=char, 6=float, 7=double, 8=byte, 9=short, 10=int, 11=long)
vl]*        Value (size depends on type)

u2          Number of static fields
[id         Static field name (UTF8 reference)
ty          Type
vl]*        Value

u2          Number of instance fields
[id         Instance field name (UTF8 reference)
u1]*        Type
```

#### HPROF_GC_INSTANCE_DUMP

Dump of a normal object:

```bash
id          Object ID
u4          Allocation stack trace serial number
id          Class object ID
u4          Number of bytes that follow
[u1]*       Instance field values (class, followed by super, super's super ...)
```

#### HPROF_GC_OBJ_ARRAY_DUMP

Dump of a object array:

```bash
id          Array object ID
u4          Allocation stack trace serial number
u4          Number of elements
id          Array class ID
[id]*       Array elements (object IDs)
```

#### HPROF_GC_PRIM_ARRAY_DUMP

Dump of a primitive array:

```bash
id          Array object ID
u4          Allocation stack trace serial number
u4          Number of elements
u1          4=boolean, 5=char, 6=float, 7=double, 8=byte, 9=short, 10=int, 11=long
[u1]*       Array elements (raw bytes)
```

### Profiling Records

#### HPROF_CPU_SAMPLE S - CPU Sampling Data

CPU profiling information:

```bash
u4      Total number of samples
u4      Number of traces
[u4     Number of Samples
u4]*    Stack trace serial number
```

#### HPROF_CONTROL_SETTINGS - Profiler Settings

Profiler configuration:

```bash
u4  flags                 // 0x00000001: allocation tracing on/off
                          // 0x00000002: CPU sampling on/off
u2  stack_trace_depth     // Maximum stack trace depth
```

#### HPROF_HEAP_DUMP_END

Marks the end of a segmented heap dump.

## Data Types

- **u1**: Unsigned 8-bit integer
- **u2**: Unsigned 16-bit integer  
- **u4**: Unsigned 32-bit integer
- **u8**: Unsigned 64-bit integer
- **i4**: Signed 32-bit integer
- **id**: Identifier (size specified in header)
- **ty**: Type code for primitives and objects

## Usage Notes

- Identifiers (id) are used to reference objects, strings, and other entities
- The identifier size is typically the same as the host pointer size
- Time offsets in records wrap around after about an hour
- Heap dumps can be split into segments for large heaps
- All multi-byte values are in big-endian format
- Object references use 0 to indicate null
