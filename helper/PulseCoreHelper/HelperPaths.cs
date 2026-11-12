using System;
using System.IO;
using System.Text;
using Newtonsoft.Json;

namespace PulseCoreHelper
{
    internal static class HelperPaths
    {
        internal const string PipeName = "PulseCoreHelper";
        internal const string TaskName = "PulseCoreHelper";
        internal const string MutexName = @"Global\PulseCoreHelper";

        internal static string DataDir
        {
            get
            {
                return Path.Combine(
                    Environment.GetFolderPath(Environment.SpecialFolder.CommonApplicationData),
                    "PulseCore");
            }
        }

        internal static string StatePath => Path.Combine(DataDir, "helper-state.json");
        internal static string LogPath => Path.Combine(DataDir, "helper.log");
        internal static string ReadyMarkerPath => Path.Combine(DataDir, "helper.ready");

        internal static string UserLogDir
        {
            get
            {
                var local = Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData);
                return Path.Combine(local, "PulseCore", "logs");
            }
        }

        internal static string UserLogPath => Path.Combine(UserLogDir, "helper.log");

        internal static void EnsureDataDir()
        {
            Directory.CreateDirectory(DataDir);
        }
    }

    internal static class HelperLog
    {
        private static readonly object Gate = new object();

        internal static void Info(string message) => Write("INFO", message);

        internal static void Warn(string message) => Write("WARN", message);

        internal static void Error(string message) => Write("ERROR", message);

        internal static void Error(Exception ex)
        {
            if (ex == null)
                return;
            Write("ERROR", ex.ToString());
        }

        private static void Write(string level, string message)
        {
            try
            {
                HelperPaths.EnsureDataDir();
                try { Directory.CreateDirectory(HelperPaths.UserLogDir); } catch { }
                var line = DateTime.Now.ToString("yyyy-MM-dd HH:mm:ss.fff") +
                           " [" + level + "] " + message + Environment.NewLine;
                lock (Gate)
                {
                    File.AppendAllText(HelperPaths.LogPath, line, Encoding.UTF8);
                    try { File.AppendAllText(HelperPaths.UserLogPath, line, Encoding.UTF8); } catch { }
                }
            }
            catch
            {
                // Logging must never crash the helper.
            }
        }
    }

    internal static class StateStore
    {
        internal static HelperState? Load()
        {
            try
            {
                if (!File.Exists(HelperPaths.StatePath))
                    return null;
                var json = File.ReadAllText(HelperPaths.StatePath, Encoding.UTF8);
                return JsonConvert.DeserializeObject<HelperState>(json, JsonUtil.Settings);
            }
            catch (Exception ex)
            {
                HelperLog.Error("Failed to read helper-state.json: " + ex.Message);
                return null;
            }
        }

        internal static void Save(HelperState state)
        {
            HelperPaths.EnsureDataDir();
            var json = JsonConvert.SerializeObject(state, Formatting.Indented, JsonUtil.Settings);
            // UTF-8 without BOM - Go json.Unmarshal rejects U+FEFF.
            File.WriteAllText(HelperPaths.StatePath, json, new UTF8Encoding(encoderShouldEmitUTF8Identifier: false));
        }
    }
}
