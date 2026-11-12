using System;
using System.IO;
using System.Threading;

namespace PulseCoreHelper
{
    internal static class Program
    {
        private static int Main(string[] args)
        {
            var mode = ParseMode(args);
            try
            {
                switch (mode)
                {
                    case "install":
                        return InstallService.Install(args);
                    case "uninstall":
                        return InstallService.Uninstall();
                    case "probe":
                        return Probe();
                    default:
                        return Serve();
                }
            }
            catch (Exception ex)
            {
                HelperLog.Error(ex);
                Console.Error.WriteLine(ex.Message);
                return 1;
            }
        }

        private static string ParseMode(string[] args)
        {
            if (args == null || args.Length == 0)
                return "serve";
            var a = args[0].Trim();
            if (a.StartsWith("--", StringComparison.Ordinal))
                a = a.Substring(2);
            else if (a.StartsWith("-", StringComparison.Ordinal) || a.StartsWith("/", StringComparison.Ordinal))
                a = a.Substring(1);
            a = a.ToLowerInvariant();
            if (a == "install" || a == "uninstall" || a == "probe" || a == "serve")
                return a;
            return "serve";
        }

        private static int Probe()
        {
            using (var sampler = new HardwareSampler())
            {
                var json = JsonUtil.ToLine(sampler.Sample());
                Console.WriteLine(json);
                return 0;
            }
        }

        private static int Serve()
        {
            if (!InstallService.IsAdministrator())
            {
                HelperLog.Error("serve requires elevation");
                Console.Error.WriteLine("PulseCoreHelper serve requires Administrator privileges.");
                return 2;
            }

            bool created;
            using (var mutex = new Mutex(true, HelperPaths.MutexName, out created))
            {
                if (!created)
                {
                    HelperLog.Info("Another PulseCoreHelper instance is already running.");
                    return 0;
                }

                NativeConsole.Hide();
                var state = StateStore.Load();
                // Rewrite state without UTF-8 BOM so the unelevated Go UI can json.Unmarshal it.
                if (state != null)
                {
                    try { StateStore.Save(state); }
                    catch (Exception ex) { HelperLog.Warn("rewrite state: " + ex.Message); }
                }
                var sid = state != null && !string.IsNullOrEmpty(state.UserSid)
                    ? state.UserSid
                    : InstallService.CurrentUserSid();

                if (!SecureInstall.VerifyInstalledIntegrity(out var reason))
                {
                    HelperLog.Error("Refusing to serve - integrity failed: " + reason);
                    Console.Error.WriteLine("Helper integrity check failed: " + reason);
                    return 3;
                }

                using (var sampler = new HardwareSampler())
                using (var host = new NamedPipeHost(sampler, sid))
                {
                    try
                    {
                        File.WriteAllText(HelperPaths.ReadyMarkerPath, DateTime.UtcNow.ToString("o"));
                    }
                    catch (Exception ex)
                    {
                        HelperLog.Warn("ready marker: " + ex.Message);
                    }
                    HelperLog.Info("Serving pipe " + HelperPaths.PipeName + " for SID " + sid);
                    host.Run();
                }

                HelperLog.Info("Helper stopped.");
                GC.KeepAlive(mutex);
                return 0;
            }
        }
    }
}
