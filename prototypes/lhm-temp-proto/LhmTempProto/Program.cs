using System.Diagnostics;
using System.Management;
using System.Runtime.Versioning;
using System.Security.Principal;
using System.ServiceProcess;
using System.Text;
using LibreHardwareMonitor.Hardware;

[SupportedOSPlatform("windows")]
internal static class Program
{
    private static int Main(string[] args)
    {
        var outPath = args.Length > 0
            ? args[0]
            : Path.Combine(AppContext.BaseDirectory, "lhm-proto-result.txt");

        var sb = new StringBuilder();
        void Line(string s = "")
        {
            Console.WriteLine(s);
            sb.AppendLine(s);
        }

        try
        {
            Line("=== PulseCore LHM Step-0 prototype ===");
            Line($"Time: {DateTime.Now:yyyy-MM-dd HH:mm:ss}");
            Line($"Machine: {Environment.MachineName}");
            Line($"OS: {Environment.OSVersion}");
            Line($"Process: {(Environment.Is64BitProcess ? "x64" : "x86")}  Admin: {IsAdmin()}");
            Line($"LibreHardwareMonitorLib: {typeof(Computer).Assembly.GetName().Version}");
            Line();

            ReportSecurity(Line);
            ReportDrivers(Line);
            Line();

            if (!IsAdmin())
            {
                Line("ERROR: not elevated. Re-run as Administrator.");
                File.WriteAllText(outPath, sb.ToString(), Encoding.UTF8);
                return 2;
            }

            var computer = new Computer
            {
                IsCpuEnabled = true,
                IsMotherboardEnabled = false,
                IsGpuEnabled = false,
                IsMemoryEnabled = false,
                IsControllerEnabled = false,
                IsNetworkEnabled = false,
                IsStorageEnabled = false,
                IsBatteryEnabled = false,
                IsPsuEnabled = false,
            };

            Exception? openEx = null;
            try
            {
                computer.Open();
                Line("Computer.Open(): OK");
            }
            catch (Exception ex)
            {
                openEx = ex;
                Line($"Computer.Open(): FAILED - {ex.GetType().Name}: {ex.Message}");
            }

            if (openEx == null)
            {
                try
                {
                    // Two update passes - some AMD sensors need a settle tick.
                    computer.Accept(new UpdateVisitor());
                    Thread.Sleep(400);
                    computer.Accept(new UpdateVisitor());

                    var cpuHw = computer.Hardware.Where(h => h.HardwareType == HardwareType.Cpu).ToList();
                    if (cpuHw.Count == 0)
                    {
                        Line("CPU hardware: NONE (driver/sensor backend likely blocked or unavailable)");
                    }

                    foreach (var hw in cpuHw)
                    {
                        Line($"CPU hardware: {hw.Name}");
                        DumpSensors(hw, Line);
                        foreach (var sub in hw.SubHardware)
                        {
                            Line($"  Subhardware: {sub.Name}");
                            DumpSensors(sub, Line, indent: "    ");
                        }
                    }

                    Line("");
                    Line("--- Summary (CPU) ---");
                    SummarizeCpu(computer, Line);
                }
                finally
                {
                    computer.Close();
                }
            }

            Line();
            Line("Compare with HWiNFO (CPU package / Tctl/Tdie / clocks / load) while both run.");
            Line($"Result file: {outPath}");
        }
        catch (Exception ex)
        {
            Line($"FATAL: {ex}");
        }

        Directory.CreateDirectory(Path.GetDirectoryName(outPath)!);
        File.WriteAllText(outPath, sb.ToString(), Encoding.UTF8);
        return 0;
    }

    private static void DumpSensors(IHardware hw, Action<string> Line, string indent = "  ")
    {
        foreach (var s in hw.Sensors.OrderBy(s => s.SensorType).ThenBy(s => s.Name))
        {
            var unit = s.SensorType switch
            {
                SensorType.Temperature => " °C",
                SensorType.Clock => " MHz",
                SensorType.Load => " %",
                SensorType.Power => " W",
                _ => "",
            };
            var val = s.Value.HasValue ? $"{s.Value.Value:0.##}{unit}" : "n/a";
            Line($"{indent}[{s.SensorType}] {s.Name} = {val}");
        }
    }

    private static void SummarizeCpu(Computer computer, Action<string> Line)
    {
        float? Pick(params string[] names)
        {
            foreach (var hw in computer.Hardware.Where(h => h.HardwareType == HardwareType.Cpu))
            {
                foreach (var s in hw.Sensors)
                {
                    foreach (var n in names)
                    {
                        if (s.Name.Contains(n, StringComparison.OrdinalIgnoreCase) && s.Value.HasValue)
                            return s.Value;
                    }
                }
            }
            return null;
        }

        float? MaxOf(SensorType type, string? nameContains = null)
        {
            float? best = null;
            foreach (var hw in computer.Hardware.Where(h => h.HardwareType == HardwareType.Cpu))
            {
                foreach (var s in hw.Sensors.Where(s => s.SensorType == type && s.Value.HasValue))
                {
                    if (nameContains != null &&
                        s.Name.IndexOf(nameContains, StringComparison.OrdinalIgnoreCase) < 0)
                        continue;
                    if (best == null || s.Value > best) best = s.Value;
                }
            }
            return best;
        }

        var tctl = Pick("Tctl", "Tdie", "CPU Package", "Package");
        var coreMax = MaxOf(SensorType.Temperature, "Core");
        var load = Pick("CPU Total", "Total");
        var clock = MaxOf(SensorType.Clock, "Core");
        var power = Pick("Package", "PPT", "CPU Package");

        Line($"Tctl/Tdie/Package °C : {(tctl.HasValue ? tctl.Value.ToString("0.0") : "UNAVAILABLE")}");
        Line($"Core temp max °C     : {(coreMax.HasValue ? coreMax.Value.ToString("0.0") : "UNAVAILABLE")}");
        Line($"CPU Total load %     : {(load.HasValue ? load.Value.ToString("0.0") : "UNAVAILABLE")}");
        Line($"Core clock max MHz   : {(clock.HasValue ? clock.Value.ToString("0") : "UNAVAILABLE")}");
        Line($"Package power W      : {(power.HasValue ? power.Value.ToString("0.0") : "UNAVAILABLE")}");

        var anyTemp = computer.Hardware
            .Where(h => h.HardwareType == HardwareType.Cpu)
            .SelectMany(h => h.Sensors)
            .Any(s => s.SensorType == SensorType.Temperature && s.Value.HasValue);
        Line($"Driver/sensors usable: {(anyTemp ? "YES (CPU temperatures present)" : "NO (no CPU temperature sensors)")}");
    }

    private static void ReportSecurity(Action<string> Line)
    {
        Line("--- Windows Defender ---");
        try
        {
            using var searcher = new ManagementObjectSearcher(@"root\Microsoft\Windows\Defender", "SELECT * FROM MSFT_MpComputerStatus");
            var found = false;
            foreach (ManagementObject o in searcher.Get())
            {
                found = true;
                Line($"AMServiceEnabled          : {o["AMServiceEnabled"]}");
                Line($"AntispywareEnabled        : {o["AntispywareEnabled"]}");
                Line($"RealTimeProtectionEnabled : {o["RealTimeProtectionEnabled"]}");
                Line($"IoavProtectionEnabled     : {o["IoavProtectionEnabled"]}");
                Line($"BehaviorMonitorEnabled    : {o["BehaviorMonitorEnabled"]}");
            }
            if (!found) Line("(MSFT_MpComputerStatus empty - fallback below)");
        }
        catch (Exception ex)
        {
            Line($"Defender WMI failed: {ex.Message}");
            try
            {
                var psi = new ProcessStartInfo("powershell",
                    "-NoProfile -Command \"Get-MpComputerStatus | Select-Object AMServiceEnabled,AntispywareEnabled,RealTimeProtectionEnabled | Format-List\"")
                {
                    RedirectStandardOutput = true,
                    UseShellExecute = false,
                    CreateNoWindow = true,
                };
                using var p = Process.Start(psi)!;
                Line(p.StandardOutput.ReadToEnd().Trim());
                p.WaitForExit(8000);
            }
            catch (Exception ex2)
            {
                Line($"Defender fallback failed: {ex2.Message}");
            }
        }

        Line("");
        Line("--- Memory Integrity / VBS / HVCI ---");
        try
        {
            using var searcher = new ManagementObjectSearcher(@"root\Microsoft\Windows\DeviceGuard", "SELECT * FROM Win32_DeviceGuard");
            foreach (ManagementObject o in searcher.Get())
            {
                Line($"VirtualizationBasedSecurityStatus : {o["VirtualizationBasedSecurityStatus"]} (0=Off,1=Enabled,2=Running)");
                Line($"SecurityServicesConfigured        : {FmtArr(o["SecurityServicesConfigured"])}");
                Line($"SecurityServicesRunning           : {FmtArr(o["SecurityServicesRunning"])} (1=Credential Guard, 2=HVCI/Memory Integrity, 3=System Guard Secure Launch)");
            }
        }
        catch (Exception ex)
        {
            Line($"DeviceGuard WMI failed: {ex.Message}");
        }

        try
        {
            using var k = Microsoft.Win32.Registry.LocalMachine.OpenSubKey(
                @"SYSTEM\CurrentControlSet\Control\DeviceGuard\Scenarios\HypervisorEnforcedCodeIntegrity");
            if (k == null)
                Line("HVCI registry key: absent (Memory Integrity typically OFF)");
            else
                Line($"HVCI Enabled registry          : {k.GetValue("Enabled")}");
        }
        catch (Exception ex)
        {
            Line($"HVCI registry read failed: {ex.Message}");
        }

        try
        {
            using var k = Microsoft.Win32.Registry.LocalMachine.OpenSubKey(
                @"SYSTEM\CurrentControlSet\Control\CI\Config");
            Line($"VulnerableDriverBlocklistEnable: {k?.GetValue("VulnerableDriverBlocklistEnable") ?? "(absent)"}");
        }
        catch (Exception ex)
        {
            Line($"CI Config read failed: {ex.Message}");
        }
    }

    private static void ReportDrivers(Action<string> Line)
    {
        Line("");
        Line("--- Kernel drivers (LHM-related) ---");
        Line("LibreHardwareMonitorLib 0.9.6 uses PawnIO (not legacy WinRing0).");
        foreach (var name in new[] { "PawnIO", "WinRing0", "WinRing0x64", "WinRing0_1_2_0", "inpoutx64" })
        {
            try
            {
                using var sc = new ServiceController(name);
                Line($"{name}: Status={sc.Status}, StartType={sc.StartType}");
            }
            catch
            {
                Line($"{name}: not installed / not found as service");
            }
        }

        try
        {
            using var searcher = new ManagementObjectSearcher(
                "SELECT Name, State, PathName FROM Win32_SystemDriver WHERE Name LIKE '%Pawn%' OR Name LIKE '%WinRing%' OR Name LIKE '%inpout%'");
            foreach (ManagementObject o in searcher.Get())
                Line($"WMI driver: {o["Name"]}  State={o["State"]}  Path={o["PathName"]}");
        }
        catch (Exception ex)
        {
            Line($"Win32_SystemDriver query failed: {ex.Message}");
        }
    }

    private static string FmtArr(object? v)
    {
        if (v is Array a) return string.Join(",", a.Cast<object>());
        return v?.ToString() ?? "";
    }

    private static bool IsAdmin()
    {
        using var id = WindowsIdentity.GetCurrent();
        var p = new WindowsPrincipal(id);
        return p.IsInRole(WindowsBuiltInRole.Administrator);
    }

    private sealed class UpdateVisitor : IVisitor
    {
        public void VisitComputer(IComputer computer) => computer.Traverse(this);
        public void VisitHardware(IHardware hardware)
        {
            hardware.Update();
            foreach (var sub in hardware.SubHardware)
                sub.Accept(this);
        }
        public void VisitSensor(ISensor sensor) { }
        public void VisitParameter(IParameter parameter) { }
    }
}
