using System.Collections.Generic;
using Xunit;

namespace PulseCoreHelper.Tests
{
    public sealed class ReadingsMapperTests
    {
        [Fact]
        public void MapCpu_AmdTctlTdieAndCcd()
        {
            var sensors = new[]
            {
                new NamedSensor("Core (Tctl/Tdie)", "Temperature", 56.5),
                new NamedSensor("CCD1 (Tdie)", "Temperature", 51.75),
                new NamedSensor("CCD2 (Tdie)", "Temperature", 49.5),
                new NamedSensor("SoC", "Temperature", 39.5),
                new NamedSensor("CPU Total", "Load", 66.35),
                new NamedSensor("Core #1", "Clock", 3993),
                new NamedSensor("Core #2", "Clock", 3993),
                new NamedSensor("Core #1 (Effective)", "Clock", 1602),
                new NamedSensor("Bus Speed", "Clock", 99.8),
                new NamedSensor("Package", "Power", 51.76),
                new NamedSensor("Core #1 (SMU)", "Power", 3.22)
            };

            var cpu = ReadingsMapper.MapCpu("AMD Ryzen 5 3600", sensors);

            Assert.Equal("AMD Ryzen 5 3600", cpu.Name);
            Assert.Equal(56.5, cpu.TctlC);
            Assert.Equal(56.5, cpu.TdieC);
            Assert.Equal(new[] { 51.75, 49.5 }, cpu.CcdTempsC);
            Assert.Equal(66.35, cpu.LoadPercent);
            Assert.Equal(3993, cpu.ClockMHz);
            Assert.Equal(51.76, cpu.PowerW);
            Assert.True(cpu.Ok);
        }

        [Fact]
        public void MapCpu_IntelPackageFallback_DoesNotInvent()
        {
            var sensors = new[]
            {
                new NamedSensor("CPU Package", "Temperature", 72.0),
                new NamedSensor("CPU Total", "Load", 12.5),
                new NamedSensor("CPU Core #1", "Clock", 4100),
                new NamedSensor("CPU Core #2", "Clock", 3900),
                new NamedSensor("CPU Package", "Power", 88.0)
            };

            var cpu = ReadingsMapper.MapCpu("Intel Core i7", sensors);

            Assert.Equal(72.0, cpu.TctlC);
            Assert.Equal(72.0, cpu.TdieC);
            Assert.Empty(cpu.CcdTempsC);
            Assert.Equal(12.5, cpu.LoadPercent);
            Assert.Equal(4000, cpu.ClockMHz);
            Assert.Equal(88.0, cpu.PowerW);
            Assert.True(cpu.Ok);
        }

        [Fact]
        public void MapCpu_ZeroPlaceholdersAreNotReadings()
        {
            var cpu = ReadingsMapper.MapCpu("AMD Ryzen 5 3600", new[]
            {
                new NamedSensor("Core (Tctl/Tdie)", "Temperature", 0),
                new NamedSensor("Package", "Power", 0),
                new NamedSensor("Core #1", "Clock", 0),
                new NamedSensor("CPU Total", "Load", 25.0)
            });

            Assert.Null(cpu.TctlC);
            Assert.Null(cpu.TdieC);
            Assert.Null(cpu.PowerW);
            Assert.Null(cpu.ClockMHz);
            Assert.Equal(25.0, cpu.LoadPercent);
            Assert.False(cpu.Ok);
        }

        [Fact]
        public void MapCpu_MissingSensorsStayNull()
        {
            var cpu = ReadingsMapper.MapCpu("Unknown", new NamedSensor[0]);
            Assert.Null(cpu.TctlC);
            Assert.Null(cpu.TdieC);
            Assert.Null(cpu.LoadPercent);
            Assert.Null(cpu.ClockMHz);
            Assert.Null(cpu.PowerW);
            Assert.Empty(cpu.CcdTempsC);
            Assert.False(cpu.Ok);
        }

        [Fact]
        public void ApplyDiskSensors_RemainingLifeAndTemp()
        {
            var disk = new DiskReading { Model = "Samsung SSD" };
            ReadingsMapper.ApplyDiskSensors(disk, new[]
            {
                new NamedSensor("Temperature", "Temperature", 34.0),
                new NamedSensor("Remaining Life", "Level", 97.0),
                new NamedSensor("Power On Hours", "Data", 1234),
                new NamedSensor("Total Bytes Written", "Data", 500000000000)
            });

            Assert.Equal(34.0, disk.TempC);
            Assert.True(disk.TempOk);
            Assert.Equal(97.0, disk.HealthPercent);
            Assert.True(disk.HealthOk);
            Assert.Equal(1234, disk.PowerOnHours);
            Assert.Equal(500000000000, disk.TotalWritesBytes);
            Assert.Equal("good", disk.Status);
        }

        [Fact]
        public void ApplyDiskSensors_DoesNotInventHealth()
        {
            var disk = new DiskReading();
            ReadingsMapper.ApplyDiskSensors(disk, new[]
            {
                new NamedSensor("Used Space", "Load", 80.0)
            });
            Assert.Null(disk.TempC);
            Assert.False(disk.TempOk);
            Assert.Null(disk.HealthPercent);
            Assert.False(disk.HealthOk);
            Assert.Equal("unknown", disk.Status);
            Assert.Empty(disk.Attrs);
        }

        [Fact]
        public void StatusFromHealth_Boundaries()
        {
            Assert.Equal("unknown", ReadingsMapper.StatusFromHealth(null));
            Assert.Equal("bad", ReadingsMapper.StatusFromHealth(9.9));
            Assert.Equal("caution", ReadingsMapper.StatusFromHealth(10));
            Assert.Equal("caution", ReadingsMapper.StatusFromHealth(49.9));
            Assert.Equal("good", ReadingsMapper.StatusFromHealth(50));
        }

        [Fact]
        public void StatusFromToolkit_MapsKnownValues()
        {
            Assert.Equal("good", ReadingsMapper.StatusFromToolkit("Good"));
            Assert.Equal("caution", ReadingsMapper.StatusFromToolkit("Caution"));
            Assert.Equal("bad", ReadingsMapper.StatusFromToolkit("Bad"));
            Assert.Equal("unknown", ReadingsMapper.StatusFromToolkit("Nope"));
        }
    }
}
