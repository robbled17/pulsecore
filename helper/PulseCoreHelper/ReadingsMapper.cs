using System;
using System.Collections.Generic;
using System.Text.RegularExpressions;

namespace PulseCoreHelper
{
    /// <summary>
    /// Pure mapping from named sensor lists to CPU/disk readings.
    /// Does not invent numbers: missing sensors stay null/false/empty.
    /// </summary>
    public static class ReadingsMapper
    {
        private static readonly Regex CoreClockRegex = new Regex(
            @"^(?:CPU\s+)?Core\s+#?\d+$",
            RegexOptions.IgnoreCase | RegexOptions.CultureInvariant | RegexOptions.Compiled);

        public static CpuReading MapCpu(string? cpuName, IEnumerable<NamedSensor>? sensors)
        {
            var reading = new CpuReading
            {
                Name = cpuName ?? "",
                CcdTempsC = new List<double>()
            };

            if (sensors == null)
                return reading;

            double? tctl = null;
            double? tdie = null;
            double? packageTemp = null;
            double? cpuTemp = null;
            var ccd = new List<KeyValuePair<string, double>>();
            double? load = null;
            var clocks = new List<double>();
            double? packagePower = null;

            foreach (var s in sensors)
            {
                if (s == null || !s.Value.HasValue)
                    continue;

                var name = s.Name ?? "";
                var type = s.SensorType ?? "";
                var value = s.Value.Value;

                if (IsType(type, "Temperature"))
                {
                    if (value <= 0)
                        continue;
                    if (Contains(name, "CCD"))
                    {
                        ccd.Add(new KeyValuePair<string, double>(name, value));
                        continue;
                    }

                    if (Contains(name, "Tctl") && !tctl.HasValue)
                        tctl = value;

                    if (Contains(name, "Tdie") && !Contains(name, "CCD") && !tdie.HasValue)
                        tdie = value;

                    if (IsPackageTempName(name) && !packageTemp.HasValue)
                        packageTemp = value;
                    else if (IsGenericCpuTempName(name) && !cpuTemp.HasValue)
                        cpuTemp = value;

                    continue;
                }

                if (IsType(type, "Load"))
                {
                    if (EqualsOrd(name, "CPU Total") && !load.HasValue)
                        load = value;
                    else if (!load.HasValue && Contains(name, "CPU Total"))
                        load = value;
                    continue;
                }

                if (IsType(type, "Clock"))
                {
                    if (value <= 0)
                        continue;
                    if (Contains(name, "Effective") ||
                        Contains(name, "Bus Speed") ||
                        Contains(name, "Fabric") ||
                        Contains(name, "Memory") ||
                        Contains(name, "Uncore") ||
                        Contains(name, "Average"))
                    {
                        continue;
                    }

                    if (CoreClockRegex.IsMatch(name.Trim()))
                        clocks.Add(value);
                    continue;
                }

                if (IsType(type, "Power"))
                {
                    if (value <= 0)
                        continue;
                    if (IsPackagePowerName(name) && !packagePower.HasValue)
                        packagePower = value;
                }
            }

            if (!tctl.HasValue)
                tctl = packageTemp ?? cpuTemp;

            if (!tdie.HasValue)
                tdie = tctl;

            ccd.Sort((a, b) => string.Compare(a.Key, b.Key, StringComparison.OrdinalIgnoreCase));
            foreach (var item in ccd)
                reading.CcdTempsC.Add(item.Value);

            reading.TctlC = tctl;
            reading.TdieC = tdie;
            reading.LoadPercent = load;
            reading.ClockMHz = AverageOrNull(clocks);
            reading.PowerW = packagePower;
            reading.Ok = tctl.HasValue || tdie.HasValue;
            return reading;
        }

        public static void ApplyDiskSensors(DiskReading disk, IEnumerable<NamedSensor>? sensors)
        {
            if (disk == null || sensors == null)
                return;

            double? temp = disk.TempC;
            double? health = disk.HealthPercent;
            long? hours = disk.PowerOnHours;
            long? writes = disk.TotalWritesBytes;

            foreach (var s in sensors)
            {
                if (s == null || !s.Value.HasValue)
                    continue;

                var name = s.Name ?? "";
                var type = s.SensorType ?? "";
                var value = s.Value.Value;

                if (IsType(type, "Temperature") && !temp.HasValue)
                {
                    if (value <= 0)
                        continue;
                    if (EqualsOrd(name, "Temperature") || Contains(name, "Temperature"))
                        temp = value;
                    continue;
                }

                if ((IsType(type, "Level") || IsType(type, "Data") || IsType(type, "Factor")) &&
                    !health.HasValue)
                {
                    if (Contains(name, "Remaining Life") ||
                        Contains(name, "Media Wearout") ||
                        Contains(name, "Available Spare") && !Contains(name, "Threshold"))
                    {
                        health = value;
                    }
                    else if (Contains(name, "Percentage Used"))
                    {
                        health = 100.0 - value;
                    }
                }

                if (!hours.HasValue && Contains(name, "Power On Hours"))
                    hours = ToLong(value);

                if (!writes.HasValue)
                {
                    if (Contains(name, "Total Bytes Written") || Contains(name, "Host Writes"))
                    {
                        var n = ToLong(value);
                        if (n.HasValue)
                        {
                            // LHM "Total Bytes Written" is bytes; "Host Writes" is often GB.
                            if (Contains(name, "Total Bytes Written") || n.Value > 1_000_000_000L)
                                writes = n;
                            else
                                writes = n.Value * 1_000_000_000L;
                        }
                    }
                    else if (Contains(name, "Data Units Written"))
                    {
                        var n = ToLong(value);
                        if (n.HasValue)
                            writes = n.Value * 512000L;
                    }
                }
            }

            disk.TempC = temp;
            disk.TempOk = temp.HasValue;
            disk.HealthPercent = health;
            disk.HealthOk = health.HasValue;
            disk.PowerOnHours = hours;
            disk.TotalWritesBytes = writes;
            if (string.IsNullOrEmpty(disk.Status) || disk.Status == "unknown")
                disk.Status = StatusFromHealth(health);
        }

        public static string StatusFromHealth(double? healthPercent)
        {
            if (!healthPercent.HasValue)
                return "unknown";
            if (healthPercent.Value < 10)
                return "bad";
            if (healthPercent.Value < 50)
                return "caution";
            return "good";
        }

        public static string StatusFromToolkit(string? diskStatus)
        {
            if (string.IsNullOrWhiteSpace(diskStatus))
                return "unknown";
            switch (diskStatus!.Trim().ToLowerInvariant())
            {
                case "good":
                    return "good";
                case "caution":
                    return "caution";
                case "bad":
                    return "bad";
                default:
                    return "unknown";
            }
        }

        private static bool IsPackageTempName(string name)
        {
            return EqualsOrd(name, "CPU Package") ||
                   EqualsOrd(name, "Package") ||
                   Contains(name, "CPU Package");
        }

        private static bool IsGenericCpuTempName(string name)
        {
            return EqualsOrd(name, "CPU") ||
                   EqualsOrd(name, "CPU Core") ||
                   EqualsOrd(name, "Core");
        }

        private static bool IsPackagePowerName(string name)
        {
            if (Contains(name, "SMU") || Contains(name, "Core #"))
                return false;
            return EqualsOrd(name, "Package") ||
                   EqualsOrd(name, "CPU Package") ||
                   EqualsOrd(name, "PPT") ||
                   Contains(name, "Package");
        }

        private static double? AverageOrNull(List<double> values)
        {
            if (values == null || values.Count == 0)
                return null;
            double sum = 0;
            foreach (var v in values)
                sum += v;
            return sum / values.Count;
        }

        private static long? ToLong(double value)
        {
            if (double.IsNaN(value) || double.IsInfinity(value) || value < 0)
                return null;
            if (value > long.MaxValue)
                return long.MaxValue;
            return Convert.ToInt64(Math.Floor(value));
        }

        private static bool IsType(string type, string expected)
        {
            return string.Equals(type, expected, StringComparison.OrdinalIgnoreCase);
        }

        private static bool Contains(string name, string token)
        {
            return name.IndexOf(token, StringComparison.OrdinalIgnoreCase) >= 0;
        }

        private static bool EqualsOrd(string a, string b)
        {
            return string.Equals(a, b, StringComparison.OrdinalIgnoreCase);
        }

    }
}
