using System;
using System.Collections.Generic;
using System.Globalization;
using System.Management;
using System.Reflection;
using System.ServiceProcess;
using DiskInfoToolkit;
using DiskInfoToolkit.Enums.Interop;
using DiskInfoToolkit.Interop.Structures;
using LibreHardwareMonitor.Hardware;

namespace PulseCoreHelper
{
    internal sealed class HardwareSampler : IDisposable
    {
        private readonly object _gate = new object();
        private Computer? _computer;
        private bool _opened;
        private bool _diskToolkitOk = true;
        private string _openError = "";

        internal HardwareSampler()
        {
            TryOpenComputer();
        }

        internal HelperResponse Sample()
        {
            var ts = DateTime.UtcNow.ToString("o", CultureInfo.InvariantCulture);
            var driver = DetectDriver();
            var cpu = new CpuReading();
            var disks = new List<DiskReading>();

            lock (_gate)
            {
                if (_computer != null && _opened)
                {
                    try
                    {
                        _computer.Accept(UpdateVisitor.Instance);
                    }
                    catch (Exception ex)
                    {
                        HelperLog.Error("Hardware update failed: " + ex.Message);
                        if (string.IsNullOrEmpty(_openError))
                            _openError = ex.Message;
                    }
                }

                cpu = ReadCpu(_computer);
                disks = ReadDisks(_computer);
            }

            if (!cpu.Ok)
            {
                if (!driver.PawnioPresent)
                {
                    driver.Blocked = true;
                    if (string.IsNullOrEmpty(driver.BlockedReason))
                        driver.BlockedReason = "PawnIO driver not installed";
                    driver.Backend = "none";
                }
                else if (!driver.PawnioRunning)
                {
                    driver.Blocked = true;
                    if (string.IsNullOrEmpty(driver.BlockedReason))
                        driver.BlockedReason = "PawnIO driver not running";
                    driver.Backend = "none";
                }
                else
                {
                    driver.Blocked = true;
                    driver.BlockedReason = string.IsNullOrEmpty(_openError)
                        ? "CPU temperature sensors unavailable"
                        : _openError;
                    driver.Backend = "PawnIO";
                }
            }
            else
            {
                driver.Blocked = false;
                driver.BlockedReason = "";
                if (driver.PawnioRunning)
                    driver.Backend = "PawnIO";
            }

            return new HelperResponse
            {
                V = 1,
                Ok = true,
                Driver = driver,
                Cpu = cpu,
                Disks = disks,
                Ts = ts
            };
        }

        internal static DriverInfo DetectDriver()
        {
            var info = new DriverInfo
            {
                Backend = "none",
                BlockedReason = ""
            };

            try
            {
                using (var sc = new ServiceController("PawnIO"))
                {
                    info.PawnioPresent = true;
                    try
                    {
                        info.PawnioRunning = sc.Status == ServiceControllerStatus.Running;
                    }
                    catch (Exception ex)
                    {
                        info.PawnioRunning = false;
                        info.BlockedReason = ex.Message;
                    }
                }
            }
            catch (InvalidOperationException)
            {
                info.PawnioPresent = false;
                info.PawnioRunning = false;
                info.Blocked = true;
                info.BlockedReason = "PawnIO driver not installed";
            }
            catch (Exception ex)
            {
                info.PawnioPresent = false;
                info.PawnioRunning = false;
                info.Blocked = true;
                info.BlockedReason = ex.Message;
            }

            if (info.PawnioPresent && !info.PawnioRunning)
            {
                info.Blocked = true;
                if (string.IsNullOrEmpty(info.BlockedReason))
                    info.BlockedReason = "PawnIO driver not running";
            }

            info.Backend = info.PawnioRunning ? "PawnIO" : "none";
            return info;
        }

        private void TryOpenComputer()
        {
            try
            {
                _computer = new Computer
                {
                    IsCpuEnabled = true,
                    IsMotherboardEnabled = false,
                    IsGpuEnabled = false,
                    IsMemoryEnabled = false,
                    IsControllerEnabled = false,
                    IsNetworkEnabled = false,
                    IsStorageEnabled = true,
                    IsBatteryEnabled = false,
                    IsPsuEnabled = false
                };
                _computer.Open();
                _opened = true;
                try
                {
                    _computer.Accept(UpdateVisitor.Instance);
                    System.Threading.Thread.Sleep(400);
                    _computer.Accept(UpdateVisitor.Instance);
                }
                catch (Exception ex)
                {
                    HelperLog.Warn("Initial sensor update failed: " + ex.Message);
                }

                TryReloadDiskToolkit();
            }
            catch (Exception ex)
            {
                HelperLog.Error("Computer.Open with storage failed: " + ex.Message);
                _openError = ex.Message;
                TryOpenCpuOnly();
            }
        }

        private void TryOpenCpuOnly()
        {
            CloseComputer();
            try
            {
                _computer = new Computer
                {
                    IsCpuEnabled = true,
                    IsStorageEnabled = false
                };
                _computer.Open();
                _opened = true;
                HelperLog.Warn("Opened LibreHardwareMonitor with CPU only (storage disabled).");
            }
            catch (Exception ex)
            {
                HelperLog.Error("Computer.Open CPU-only failed: " + ex.Message);
                if (string.IsNullOrEmpty(_openError))
                    _openError = ex.Message;
                CloseComputer();
            }
        }

        private void TryReloadDiskToolkit()
        {
            if (!_diskToolkitOk)
                return;
            try
            {
                StorageManager.ReloadStorages();
            }
            catch (Exception ex)
            {
                HelperLog.Warn("DiskInfoToolkit ReloadStorages: " + ex.Message);
            }
        }

        private static CpuReading ReadCpu(Computer? computer)
        {
            string name = "";
            var sensors = new List<NamedSensor>();
            if (computer != null)
            {
                foreach (var hw in computer.Hardware)
                {
                    if (hw.HardwareType != HardwareType.Cpu)
                        continue;
                    if (string.IsNullOrEmpty(name))
                        name = hw.Name ?? "";
                    CollectSensors(hw, sensors);
                }
            }

            return ReadingsMapper.MapCpu(name, sensors);
        }

        private List<DiskReading> ReadDisks(Computer? computer)
        {
            var lhmByModel = new Dictionary<string, List<NamedSensor>>(StringComparer.OrdinalIgnoreCase);
            var lhmNames = new Dictionary<string, string>(StringComparer.OrdinalIgnoreCase);
            if (computer != null)
            {
                foreach (var hw in computer.Hardware)
                {
                    if (hw.HardwareType != HardwareType.Storage)
                        continue;
                    var model = hw.Name ?? "";
                    var sensors = new List<NamedSensor>();
                    CollectSensors(hw, sensors);
                    var key = NormalizeModel(model);
                    if (string.IsNullOrEmpty(key))
                        key = hw.Identifier != null ? hw.Identifier.ToString() : Guid.NewGuid().ToString();
                    lhmByModel[key] = sensors;
                    lhmNames[key] = model;
                }
            }

            var disks = new List<DiskReading>();
            var usedLhm = new HashSet<string>(StringComparer.OrdinalIgnoreCase);

            if (_diskToolkitOk)
            {
                try
                {
                    var storages = StorageManager.Storages;
                    if (storages != null)
                    {
                        foreach (var storage in storages)
                        {
                            if (storage == null)
                                continue;
                            try
                            {
                                storage.Update();
                            }
                            catch (Exception ex)
                            {
                                HelperLog.Warn("DiskInfoToolkit Update failed for " +
                                               (storage.Model ?? "?") + ": " + ex.Message);
                            }

                            var disk = MapToolkitStorage(storage);
                            var key = NormalizeModel(disk.Model);
                            List<NamedSensor>? sensors = null;
                            if (!string.IsNullOrEmpty(key) && lhmByModel.TryGetValue(key, out sensors))
                                usedLhm.Add(key);
                            else
                            {
                                sensors = FindSensorsByPartialModel(lhmByModel, disk.Model, usedLhm);
                            }

                            ReadingsMapper.ApplyDiskSensors(disk, sensors);
                            disks.Add(disk);
                        }
                    }
                }
                catch (Exception ex)
                {
                    _diskToolkitOk = false;
                    HelperLog.Error("DiskInfoToolkit StorageManager failed: " + ex.Message);
                }
            }

            foreach (var kv in lhmByModel)
            {
                if (usedLhm.Contains(kv.Key))
                    continue;
                var disk = new DiskReading
                {
                    Model = lhmNames.ContainsKey(kv.Key) ? lhmNames[kv.Key] : kv.Key,
                    Status = "unknown"
                };
                ReadingsMapper.ApplyDiskSensors(disk, kv.Value);
                disks.Add(disk);
            }

            if (disks.Count > 0)
                FillMissingLetters(disks);

            return disks;
        }

        private static DiskReading MapToolkitStorage(Storage storage)
        {
            var disk = new DiskReading
            {
                Model = storage.Model ?? "",
                Serial = storage.SerialNumber ?? "",
                Interface = FormatBus(storage.BusType),
                Letter = FirstDriveLetter(storage),
                Status = "unknown"
            };

            ApplyMediaKind(storage, disk);

            try
            {
                var smart = storage.Smart;
                if (smart != null)
                {
                    if (smart.Temperature.HasValue)
                        disk.TempC = Convert.ToDouble(smart.Temperature.Value, CultureInfo.InvariantCulture);
                    // Life % is SSD/NVMe wear - do not treat as HDD health.
                    if (smart.Life.HasValue && disk.MediaKind != "hdd")
                        disk.HealthPercent = Convert.ToDouble(smart.Life.Value, CultureInfo.InvariantCulture);

                    disk.PowerOnHours = PickPowerOnHours(smart);
                    disk.TotalWritesBytes = HostWritesToBytes(smart.HostWrites);

                    disk.Status = ReadingsMapper.StatusFromToolkit(smart.DiskStatus.ToString());
                    disk.Attrs = MapSmartAttrs(smart);
                }
            }
            catch (Exception ex)
            {
                HelperLog.Warn("SMART map failed for " + disk.Model + ": " + ex.Message);
            }

            disk.TempOk = disk.TempC.HasValue;
            disk.HealthOk = disk.HealthPercent.HasValue;
            if (disk.Status == "unknown")
                disk.Status = ReadingsMapper.StatusFromHealth(disk.HealthPercent);
            return disk;
        }

        /// <summary>
        /// Media type without guessing SSD: NVMe → nvme; toolkit IsSSD; else IDENTIFY word 217 via reflection; else unknown/hdd.
        /// DiskInfoToolkit IsSSD is derived from Nominal Media Rotation Rate (word 217): 1 = SSD, rpm = HDD.
        /// </summary>
        private static void ApplyMediaKind(Storage storage, DiskReading disk)
        {
            disk.IsNvme = storage.IsNVMe;
            disk.IsSsd = storage.IsSSD;
            if (storage.IsNVMe)
            {
                disk.MediaKind = "nvme";
                disk.Interface = "NVMe";
                return;
            }

            int? word217 = TryReadIdentifyWord217(storage);
            disk.IdentifyWord217 = word217;
            if (word217.HasValue)
            {
                var w = word217.Value & 0xFFFF;
                if (w == 1)
                {
                    disk.MediaKind = "ssd";
                    disk.IsSsd = true;
                    return;
                }
                if (w >= 0x0401 && w <= 0xFFFE)
                {
                    disk.MediaKind = "hdd";
                    disk.IsSsd = false;
                    disk.RotationRpm = w;
                    return;
                }
            }

            // Toolkit IsSSD mirrors word 217 when available.
            if (storage.IsSSD)
            {
                disk.MediaKind = "ssd";
                disk.IsSsd = true;
                return;
            }

            // Fixed rotational drive (IsSSD false, not NVMe).
            disk.MediaKind = "hdd";
            disk.IsSsd = false;
            if (!disk.RotationRpm.HasValue)
            {
                var rpm = TryQuerySpindleRpm(disk.Model, disk.Serial);
                if (rpm > 1)
                    disk.RotationRpm = rpm;
            }
        }

        private static int? TryReadIdentifyWord217(Storage storage)
        {
            try
            {
                // Prefer public ATA identify buffer if toolkit exposes it on nested types.
                var flags = BindingFlags.Instance | BindingFlags.Public | BindingFlags.NonPublic;
                object? cur = storage;
                foreach (var name in new[] { "IdentifyDevice", "Identify", "AtaIdentify", "ATAInfo" })
                {
                    var p = cur.GetType().GetProperty(name, flags);
                    if (p == null) continue;
                    var next = p.GetValue(cur);
                    if (next == null) continue;
                    cur = next;
                    var f = cur.GetType().GetField("NominalMediaRotationRate", flags);
                    if (f != null)
                    {
                        var v = f.GetValue(cur);
                        if (v is ushort us) return us;
                        if (v is short sh) return unchecked((ushort)sh);
                        if (v is int i) return i & 0xFFFF;
                    }
                    var prop = cur.GetType().GetProperty("NominalMediaRotationRate", flags);
                    if (prop != null)
                    {
                        var v = prop.GetValue(cur);
                        if (v is ushort us) return us;
                        if (v is short sh) return unchecked((ushort)sh);
                        if (v is int i) return i & 0xFFFF;
                    }
                }
            }
            catch (Exception ex)
            {
                HelperLog.Warn("IDENTIFY word 217: " + ex.Message);
            }
            return null;
        }

        private static int TryQuerySpindleRpm(string model, string serial)
        {
            try
            {
                using (var searcher = new System.Management.ManagementObjectSearcher(
                           "root\\Microsoft\\Windows\\Storage",
                           "SELECT FriendlyName, SerialNumber, SpindleSpeed, MediaType FROM MSFT_PhysicalDisk"))
                {
                    foreach (System.Management.ManagementObject o in searcher.Get())
                    {
                        using (o)
                        {
                            var name = (o["FriendlyName"] ?? "").ToString() ?? "";
                            var sn = (o["SerialNumber"] ?? "").ToString() ?? "";
                            var match = (!string.IsNullOrEmpty(model) && name.IndexOf(model.Trim(), StringComparison.OrdinalIgnoreCase) >= 0) ||
                                        (!string.IsNullOrEmpty(serial) && sn.IndexOf(serial.Trim(), StringComparison.OrdinalIgnoreCase) >= 0) ||
                                        (!string.IsNullOrEmpty(serial) && serial.IndexOf(sn.Trim(), StringComparison.OrdinalIgnoreCase) >= 0);
                            if (!match) continue;
                            var rpmObj = o["SpindleSpeed"];
                            if (rpmObj == null) continue;
                            var rpm = Convert.ToInt32(rpmObj, CultureInfo.InvariantCulture);
                            if (rpm > 1 && rpm < 0xFFFF)
                                return rpm;
                        }
                    }
                }
            }
            catch (Exception ex)
            {
                HelperLog.Warn("MSFT SpindleSpeed: " + ex.Message);
            }
            return 0;
        }

        private static List<SmartAttr> MapSmartAttrs(SmartInfo smart)
        {
            var list = new List<SmartAttr>();
            if (smart.SmartAttributes == null)
                return list;

            foreach (var a in smart.SmartAttributes)
            {
                if (a == null)
                    continue;
                try
                {
                    var attr = a.Attribute;
                    int id = attr.ID;
                    if (id == 0)
                        continue;
                    var name = a.Info != null ? (a.Info.Name ?? "") : "";
                    list.Add(new SmartAttr
                    {
                        Id = id,
                        Name = name,
                        Current = attr.CurrentValue,
                        Worst = attr.WorstValue,
                        Threshold = attr.Threshold,
                        Raw = FormatRaw(attr)
                    });
                }
                catch
                {
                    // Skip malformed attribute rows rather than failing the disk.
                }
            }

            return list;
        }

        private static string FormatRaw(SmartAttributeStructure attr)
        {
            try
            {
                return attr.RawValueULong.ToString(CultureInfo.InvariantCulture);
            }
            catch
            {
                if (attr.RawValue == null || attr.RawValue.Length == 0)
                    return "";
                return BitConverter.ToString(attr.RawValue).Replace("-", "");
            }
        }

        private static long? PickPowerOnHours(SmartInfo smart)
        {
            if (smart.DetectedPowerOnHours > 0)
                return (long)smart.DetectedPowerOnHours;
            if (smart.MeasuredPowerOnHours > 0)
                return (long)smart.MeasuredPowerOnHours;
            if (smart.SmartAttributes != null && smart.SmartAttributes.Count > 0)
                return (long)smart.DetectedPowerOnHours;
            return null;
        }

        private static long? HostWritesToBytes(object? hostWrites)
        {
            if (hostWrites == null)
                return null;
            try
            {
                var n = Convert.ToDouble(hostWrites, CultureInfo.InvariantCulture);
                if (double.IsNaN(n) || n < 0)
                    return null;
                // DiskInfoToolkit documents HostWrites as gigabytes.
                var bytes = n * 1000000000.0;
                if (bytes > long.MaxValue)
                    return long.MaxValue;
                return Convert.ToInt64(Math.Floor(bytes));
            }
            catch
            {
                return null;
            }
        }

        private static string FirstDriveLetter(Storage storage)
        {
            try
            {
                if (storage.Partitions == null)
                    return "";
                char? best = null;
                foreach (var p in storage.Partitions)
                {
                    if (p == null || !p.DriveLetter.HasValue)
                        continue;
                    var c = p.DriveLetter.Value;
                    if (c == '\0')
                        continue;
                    if (!best.HasValue || c < best.Value)
                        best = c;
                }

                return best.HasValue ? best.Value.ToString() : "";
            }
            catch
            {
                return "";
            }
        }

        private static string FormatBus(StorageBusType bus)
        {
            var s = bus.ToString();
            if (s.StartsWith("BusType", StringComparison.Ordinal))
                s = s.Substring("BusType".Length);
            if (string.Equals(s, "Nvme", StringComparison.OrdinalIgnoreCase))
                return "NVMe";
            if (string.Equals(s, "Sata", StringComparison.OrdinalIgnoreCase))
                return "SATA";
            if (string.Equals(s, "Usb", StringComparison.OrdinalIgnoreCase))
                return "USB";
            if (string.Equals(s, "Raid", StringComparison.OrdinalIgnoreCase))
                return "RAID";
            if (string.Equals(s, "Sas", StringComparison.OrdinalIgnoreCase))
                return "SAS";
            return s;
        }

        private static List<NamedSensor>? FindSensorsByPartialModel(
            Dictionary<string, List<NamedSensor>> lhmByModel,
            string model,
            HashSet<string> usedLhm)
        {
            var n = NormalizeModel(model);
            if (string.IsNullOrEmpty(n))
                return null;
            foreach (var kv in lhmByModel)
            {
                if (usedLhm.Contains(kv.Key))
                    continue;
                if (kv.Key.IndexOf(n, StringComparison.OrdinalIgnoreCase) >= 0 ||
                    n.IndexOf(kv.Key, StringComparison.OrdinalIgnoreCase) >= 0)
                {
                    usedLhm.Add(kv.Key);
                    return kv.Value;
                }
            }

            return null;
        }

        private static void FillMissingLetters(List<DiskReading> disks)
        {
            var need = false;
            foreach (var d in disks)
            {
                if (string.IsNullOrEmpty(d.Letter))
                {
                    need = true;
                    break;
                }
            }

            if (!need)
                return;

            Dictionary<string, string> map;
            try
            {
                map = QueryWmiDriveLetters();
            }
            catch (Exception ex)
            {
                HelperLog.Warn("WMI drive-letter lookup failed: " + ex.Message);
                return;
            }

            foreach (var d in disks)
            {
                if (!string.IsNullOrEmpty(d.Letter))
                    continue;
                var key = NormalizeModel(d.Model);
                if (!string.IsNullOrEmpty(key) && map.TryGetValue(key, out var letter))
                    d.Letter = letter;
            }
        }

        private static Dictionary<string, string> QueryWmiDriveLetters()
        {
            var result = new Dictionary<string, string>(StringComparer.OrdinalIgnoreCase);
            using (var drives = new ManagementObjectSearcher(
                       "SELECT DeviceID, Model FROM Win32_DiskDrive"))
            {
                foreach (ManagementObject drive in drives.Get())
                {
                    using (drive)
                    {
                        var model = NormalizeModel(Convert.ToString(drive["Model"]) ?? "");
                        var deviceId = Convert.ToString(drive["DeviceID"]) ?? "";
                        if (string.IsNullOrEmpty(model) || string.IsNullOrEmpty(deviceId))
                            continue;
                        var letter = FirstLogicalLetter(drive);
                        if (string.IsNullOrEmpty(letter))
                            continue;
                        if (!result.ContainsKey(model) || string.CompareOrdinal(letter, result[model]) < 0)
                            result[model] = letter;
                    }
                }
            }

            return result;
        }

        private static string FirstLogicalLetter(ManagementObject drive)
        {
            char? best = null;
            foreach (ManagementObject partition in drive.GetRelated("Win32_DiskPartition"))
            {
                using (partition)
                {
                    foreach (ManagementObject logical in partition.GetRelated("Win32_LogicalDisk"))
                    {
                        using (logical)
                        {
                            var id = Convert.ToString(logical["DeviceID"]) ?? "";
                            if (id.Length == 0)
                                continue;
                            var c = char.ToUpperInvariant(id[0]);
                            if (c < 'A' || c > 'Z')
                                continue;
                            if (!best.HasValue || c < best.Value)
                                best = c;
                        }
                    }
                }
            }

            return best.HasValue ? best.Value.ToString() : "";
        }

        private static void CollectSensors(IHardware hw, List<NamedSensor> dest)
        {
            if (hw.Sensors != null)
            {
                foreach (var s in hw.Sensors)
                {
                    dest.Add(new NamedSensor(
                        s.Name ?? "",
                        s.SensorType.ToString(),
                        s.Value.HasValue ? (double?)s.Value.Value : null));
                }
            }

            if (hw.SubHardware == null)
                return;
            foreach (var sub in hw.SubHardware)
            {
                try
                {
                    sub.Update();
                }
                catch
                {
                    // Keep going; subhardware update is best-effort.
                }

                CollectSensors(sub, dest);
            }
        }

        private static string NormalizeModel(string model)
        {
            if (string.IsNullOrEmpty(model))
                return "";
            var chars = new char[model.Length];
            var n = 0;
            foreach (var c in model)
            {
                if (char.IsLetterOrDigit(c))
                    chars[n++] = char.ToUpperInvariant(c);
            }

            return new string(chars, 0, n);
        }

        private void CloseComputer()
        {
            if (_computer == null)
                return;
            try
            {
                _computer.Close();
            }
            catch
            {
            }

            _computer = null;
            _opened = false;
        }

        public void Dispose()
        {
            lock (_gate)
                CloseComputer();
        }

        private sealed class UpdateVisitor : IVisitor
        {
            internal static readonly UpdateVisitor Instance = new UpdateVisitor();

            public void VisitComputer(IComputer computer) => computer.Traverse(this);

            public void VisitHardware(IHardware hardware)
            {
                hardware.Update();
                foreach (var sub in hardware.SubHardware)
                    sub.Accept(this);
            }

            public void VisitSensor(ISensor sensor)
            {
            }

            public void VisitParameter(IParameter parameter)
            {
            }
        }
    }
}
