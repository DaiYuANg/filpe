"""Create a sample Excel file for testing."""

from pathlib import Path

from openpyxl import Workbook

wb = Workbook()
ws = wb.active
ws.title = "Sheet1"
ws.append(["Name", "Age", "City"])
ws.append(["Alice", 30, "Beijing"])
ws.append(["Bob", 25, "Shanghai"])
ws.append(["Carol", 35, "Shenzhen"])

out = Path(__file__).parent.parent / "sample.xlsx"
wb.save(out)
print(f"Created {out}")
