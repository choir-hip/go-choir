package choir.probe;

import java.io.File;
import java.lang.reflect.InvocationTargetException;
import java.lang.reflect.Method;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Paths;

/**
 * Device proof for the APK's own Kotlin/JNI model runtime. Run under `app_process` with the
 * installed app's dex in the class path: it loads choir.autoradio.NativeModels from the APK,
 * which loads libautoradio_models.so from lib/arm64-v8a, and prints the runtime's measurements.
 * No test switch ships in the app; scripts/proofs/android-a6-model-runtime.sh runs this.
 *
 *   app_process -Djava.class.path=probe.jar:base.apk /system/bin choir.probe.ModelProbe
 *     <kind> <model-id> <model-path> <@prompt-file> <threads> <max-tokens> <report-file>
 */
public final class ModelProbe {
    public static void main(String[] args) throws Exception {
        final String kind = args[0], id = args[1], path = args[2];
        final String prompt = args[3].startsWith("@")
                ? new String(Files.readAllBytes(Paths.get(args[3].substring(1))), StandardCharsets.UTF_8)
                : args[3];
        final int threads = Integer.parseInt(args[4]), maxTokens = Integer.parseInt(args[5]);
        // app_process sends a plain process's stdout to /dev/null; the report is a file the proof reads back.
        final java.io.PrintStream report = new java.io.PrintStream(
                new java.io.FileOutputStream(args.length > 6 ? args[6] : "/data/local/tmp/a6probe.out", true), true);
        System.setOut(report);
        System.setErr(report);
        report.println("probe=start kind=" + kind + " model=" + id + " threads=" + threads);
        try {
            run(kind, id, path, prompt, threads, maxTokens);
        } catch (InvocationTargetException error) {
            Throwable cause = error.getCause();
            report.println("failed=" + cause.getClass().getName() + ": " + cause.getMessage());
            cause.printStackTrace(report);
            report.flush();
            System.exit(1);
        } catch (Throwable error) {
            report.println("failed=" + error.getClass().getName() + ": " + error.getMessage());
            error.printStackTrace(report);
            report.flush();
            System.exit(1);
        }
        report.println("probe=done");
        report.flush();
        System.exit(0);
    }

    private static void run(String kind, String id, String path, String prompt, int threads, int maxTokens) throws Exception {
        final Class<?> catalog = Class.forName("choir.autoradio.NativeModelCatalog");
        final Object definitions = catalog.getField("INSTANCE").get(null);
        final Object spec = catalog.getMethod("require", String.class).invoke(definitions, id);
        final Class<?> modelsClass = Class.forName("choir.autoradio.NativeModels");
        final Object models = modelsClass.getConstructor().newInstance();
        try {
            final Object info = modelsClass.getMethod("info").invoke(models);
            System.out.println("runtime=" + get(info, "name") + " version=" + get(info, "version")
                    + " abi=" + get(info, "abi") + " available=" + get(info, "available"));
            final long loadStart = millis();
            modelsClass.getMethod("load", spec.getClass(), File.class, int.class, int.class, int.class)
                    .invoke(models, spec, new File(path), 2048, threads, 256);
            System.out.println("load_ms=" + (millis() - loadStart) + " model=" + id + " bytes=" + new File(path).length());
            final long workStart = millis();
            if ("text".equals(kind)) {
                Method generate = null;
                for (Method method : modelsClass.getMethods()) if (method.getName().equals("generate")) generate = method;
                if (generate == null) throw new IllegalStateException("NativeModels has no generate");
                final Object result = generate.invoke(models, prompt, maxTokens, 0L, 0f, 40, 1f, null);
                System.out.println("generate_ms=" + (millis() - workStart));
                for (String field : new String[]{"text", "promptTokens", "generatedTokens", "prefillMs", "firstTokenMs", "decodeMs", "tokensPerSecond", "cancelled"}) {
                    System.out.println("gen_" + field + "=" + get(result, field));
                }
            } else {
                final float[] vector = (float[]) modelsClass.getMethod("embed", String.class).invoke(models, prompt);
                double squared = 0;
                for (float value : vector) squared += (double) value * value;
                System.out.println("embed_ms=" + (millis() - workStart));
                System.out.println("dim=" + vector.length + " norm=" + String.format("%.6f", Math.sqrt(squared))
                        + " first=" + vector[0]);
            }
            System.out.println("vmhwm_kb=" + peakRssKb());
        } finally {
            try { modelsClass.getMethod("close").invoke(models); } catch (Exception ignored) { }
        }
    }

    private static String get(Object value, String name) throws Exception {
        for (String method : new String[]{"get" + Character.toUpperCase(name.charAt(0)) + name.substring(1), "is" + Character.toUpperCase(name.charAt(0)) + name.substring(1), name}) {
            try { return String.valueOf(value.getClass().getMethod(method).invoke(value)); }
            catch (NoSuchMethodException ignored) { }
        }
        throw new NoSuchMethodException(name);
    }

    private static long millis() {
        return System.nanoTime() / 1_000_000L;
    }

    private static String peakRssKb() {
        try {
            for (String line : Files.readAllLines(Paths.get("/proc/self/status"), StandardCharsets.UTF_8)) {
                if (line.startsWith("VmHWM:")) return line.replaceAll("[^0-9]", "");
            }
        } catch (Exception ignored) { }
        return "unknown";
    }
}
