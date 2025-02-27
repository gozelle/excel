package excel

import (
	"encoding/xml"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	
	"github.com/stretchr/testify/assert"
)

func BenchmarkStreamWriter(b *testing.B) {
	file := NewFile()
	defer func() {
		if err := file.Close(); err != nil {
			b.Error(err)
		}
	}()
	row := make([]interface{}, 10)
	for colID := 0; colID < 10; colID++ {
		row[colID] = colID
	}
	
	for n := 0; n < b.N; n++ {
		streamWriter, _ := file.NewStreamWriter("Sheet1")
		for rowID := 10; rowID <= 110; rowID++ {
			cell, _ := CoordinatesToCellName(1, rowID)
			_ = streamWriter.SetRow(cell, row)
		}
	}
	
	b.ReportAllocs()
}

func TestStreamWriter(t *testing.T) {
	file := NewFile()
	streamWriter, err := file.NewStreamWriter("Sheet1")
	assert.NoError(t, err)
	
	// Test max characters in a cell
	row := make([]interface{}, 1)
	row[0] = strings.Repeat("c", TotalCellChars+2)
	assert.NoError(t, streamWriter.SetRow("A1", row))
	
	// Test leading and ending space(s) character characters in a cell
	row = make([]interface{}, 1)
	row[0] = " characters"
	assert.NoError(t, streamWriter.SetRow("A2", row))
	
	row = make([]interface{}, 1)
	row[0] = []byte("Word")
	assert.NoError(t, streamWriter.SetRow("A3", row))
	
	// Test set cell with style and rich text
	styleID, err := file.NewStyle(&Style{Font: &Font{Color: "#777777"}})
	assert.NoError(t, err)
	assert.NoError(t, streamWriter.SetRow("A4", []interface{}{
		Cell{StyleID: styleID},
		Cell{Formula: "SUM(A10,B10)", Value: " preserve space "},
	},
		RowOpts{Height: 45, StyleID: styleID}))
	assert.NoError(t, streamWriter.SetRow("A5", []interface{}{
		&Cell{StyleID: styleID, Value: "cell <>&'\""},
		&Cell{Formula: "SUM(A10,B10)"},
		[]RichTextRun{
			{Text: "Rich ", Font: &Font{Color: "2354e8"}},
			{Text: "Text", Font: &Font{Color: "e83723"}},
		},
	}))
	assert.NoError(t, streamWriter.SetRow("A6", []interface{}{time.Now()}))
	assert.NoError(t, streamWriter.SetRow("A7", nil, RowOpts{Height: 20, Hidden: true, StyleID: styleID}))
	assert.EqualError(t, streamWriter.SetRow("A8", nil, RowOpts{Height: MaxRowHeight + 1}), ErrMaxRowHeight.Error())
	
	for rowID := 10; rowID <= 51200; rowID++ {
		row := make([]interface{}, 50)
		for colID := 0; colID < 50; colID++ {
			row[colID] = rand.Intn(640000)
		}
		cell, _ := CoordinatesToCellName(1, rowID)
		assert.NoError(t, streamWriter.SetRow(cell, row))
	}
	
	assert.NoError(t, streamWriter.Flush())
	// Save spreadsheet by the given path
	assert.NoError(t, file.SaveAs(filepath.Join("test", "TestStreamWriter.xlsx")))
	
	// Test set cell column overflow
	assert.ErrorIs(t, streamWriter.SetRow("XFD51201", []interface{}{"A", "B", "C"}), ErrColumnNumber)
	assert.NoError(t, file.Close())
	
	// Test close temporary file error
	file = NewFile()
	streamWriter, err = file.NewStreamWriter("Sheet1")
	assert.NoError(t, err)
	for rowID := 10; rowID <= 25600; rowID++ {
		row := make([]interface{}, 50)
		for colID := 0; colID < 50; colID++ {
			row[colID] = rand.Intn(640000)
		}
		cell, _ := CoordinatesToCellName(1, rowID)
		assert.NoError(t, streamWriter.SetRow(cell, row))
	}
	assert.NoError(t, streamWriter.rawData.Close())
	assert.Error(t, streamWriter.Flush())
	
	streamWriter.rawData.tmp, err = os.CreateTemp(os.TempDir(), "excelize-")
	assert.NoError(t, err)
	_, err = streamWriter.rawData.Reader()
	assert.NoError(t, err)
	assert.NoError(t, streamWriter.rawData.tmp.Close())
	assert.NoError(t, os.Remove(streamWriter.rawData.tmp.Name()))
	
	// Test create stream writer with unsupported charset
	file = NewFile()
	file.Sheet.Delete("xl/worksheets/sheet1.xml")
	file.Pkg.Store("xl/worksheets/sheet1.xml", MacintoshCyrillicCharset)
	_, err = file.NewStreamWriter("Sheet1")
	assert.EqualError(t, err, "XML syntax error on line 1: invalid UTF-8")
	assert.NoError(t, file.Close())
	
	// Test read cell
	file = NewFile()
	streamWriter, err = file.NewStreamWriter("Sheet1")
	assert.NoError(t, err)
	assert.NoError(t, streamWriter.SetRow("A1", []interface{}{Cell{StyleID: styleID, Value: "Data"}}))
	assert.NoError(t, streamWriter.Flush())
	cellValue, err := file.GetCellValue("Sheet1", "A1")
	assert.NoError(t, err)
	assert.Equal(t, "Data", cellValue)
	
	// Test stream reader for a worksheet with huge amounts of data
	file, err = OpenFile(filepath.Join("test", "TestStreamWriter.xlsx"))
	assert.NoError(t, err)
	rows, err := file.Rows("Sheet1")
	assert.NoError(t, err)
	cells := 0
	for rows.Next() {
		row, err := rows.Columns()
		assert.NoError(t, err)
		cells += len(row)
	}
	assert.NoError(t, rows.Close())
	assert.Equal(t, 2559559, cells)
	// Save spreadsheet with password.
	assert.NoError(t, file.SaveAs(filepath.Join("test", "EncryptionTestStreamWriter.xlsx"), Options{Password: "password"}))
	assert.NoError(t, file.Close())
}

func TestStreamSetColWidth(t *testing.T) {
	file := NewFile()
	defer func() {
		assert.NoError(t, file.Close())
	}()
	streamWriter, err := file.NewStreamWriter("Sheet1")
	assert.NoError(t, err)
	assert.NoError(t, streamWriter.SetColWidth(3, 2, 20))
	assert.ErrorIs(t, streamWriter.SetColWidth(0, 3, 20), ErrColumnNumber)
	assert.ErrorIs(t, streamWriter.SetColWidth(MaxColumns+1, 3, 20), ErrColumnNumber)
	assert.EqualError(t, streamWriter.SetColWidth(1, 3, MaxColumnWidth+1), ErrColumnWidth.Error())
	assert.NoError(t, streamWriter.SetRow("A1", []interface{}{"A", "B", "C"}))
	assert.ErrorIs(t, streamWriter.SetColWidth(2, 3, 20), ErrStreamSetColWidth)
}

func TestStreamSetPanes(t *testing.T) {
	file, paneOpts := NewFile(), &Panes{
		Freeze:      true,
		Split:       false,
		XSplit:      1,
		YSplit:      0,
		TopLeftCell: "B1",
		ActivePane:  "topRight",
		Panes: []PaneOptions{
			{SQRef: "K16", ActiveCell: "K16", Pane: "topRight"},
		},
	}
	defer func() {
		assert.NoError(t, file.Close())
	}()
	streamWriter, err := file.NewStreamWriter("Sheet1")
	assert.NoError(t, err)
	assert.NoError(t, streamWriter.SetPanes(paneOpts))
	assert.EqualError(t, streamWriter.SetPanes(nil), ErrParameterInvalid.Error())
	assert.NoError(t, streamWriter.SetRow("A1", []interface{}{"A", "B", "C"}))
	assert.ErrorIs(t, streamWriter.SetPanes(paneOpts), ErrStreamSetPanes)
}

func TestStreamTable(t *testing.T) {
	file := NewFile()
	defer func() {
		assert.NoError(t, file.Close())
	}()
	streamWriter, err := file.NewStreamWriter("Sheet1")
	assert.NoError(t, err)
	// Test add table without table header
	assert.EqualError(t, streamWriter.AddTable("A1:C2", nil), "XML syntax error on line 2: unexpected EOF")
	// Write some rows. We want enough rows to force a temp file (>16MB)
	assert.NoError(t, streamWriter.SetRow("A1", []interface{}{"A", "B", "C"}))
	row := []interface{}{1, 2, 3}
	for r := 2; r < 10000; r++ {
		assert.NoError(t, streamWriter.SetRow(fmt.Sprintf("A%d", r), row))
	}
	
	// Write a table
	assert.NoError(t, streamWriter.AddTable("A1:C2", nil))
	assert.NoError(t, streamWriter.Flush())
	
	// Verify the table has names
	var table xlsxTable
	val, ok := file.Pkg.Load("xl/tables/table1.xml")
	assert.True(t, ok)
	assert.NoError(t, xml.Unmarshal(val.([]byte), &table))
	assert.Equal(t, "A", table.TableColumns.TableColumn[0].Name)
	assert.Equal(t, "B", table.TableColumns.TableColumn[1].Name)
	assert.Equal(t, "C", table.TableColumns.TableColumn[2].Name)
	
	assert.NoError(t, streamWriter.AddTable("A1:C1", nil))
	
	// Test add table with illegal cell reference
	assert.EqualError(t, streamWriter.AddTable("A:B1", nil), newCellNameToCoordinatesError("A", newInvalidCellNameError("A")).Error())
	assert.EqualError(t, streamWriter.AddTable("A1:B", nil), newCellNameToCoordinatesError("B", newInvalidCellNameError("B")).Error())
	// Test add table with unsupported charset content types
	file.ContentTypes = nil
	file.Pkg.Store(defaultXMLPathContentTypes, MacintoshCyrillicCharset)
	assert.EqualError(t, streamWriter.AddTable("A1:C2", nil), "XML syntax error on line 1: invalid UTF-8")
}

func TestStreamMergeCells(t *testing.T) {
	file := NewFile()
	defer func() {
		assert.NoError(t, file.Close())
	}()
	streamWriter, err := file.NewStreamWriter("Sheet1")
	assert.NoError(t, err)
	assert.NoError(t, streamWriter.MergeCell("A1", "D1"))
	// Test merge cells with illegal cell reference
	assert.EqualError(t, streamWriter.MergeCell("A", "D1"), newCellNameToCoordinatesError("A", newInvalidCellNameError("A")).Error())
	assert.NoError(t, streamWriter.Flush())
	// Save spreadsheet by the given path
	assert.NoError(t, file.SaveAs(filepath.Join("test", "TestStreamMergeCells.xlsx")))
}

func TestStreamInsertPageBreak(t *testing.T) {
	file := NewFile()
	defer func() {
		assert.NoError(t, file.Close())
	}()
	streamWriter, err := file.NewStreamWriter("Sheet1")
	assert.NoError(t, err)
	assert.NoError(t, streamWriter.InsertPageBreak("A1"))
	assert.NoError(t, streamWriter.Flush())
	// Save spreadsheet by the given path
	assert.NoError(t, file.SaveAs(filepath.Join("test", "TestStreamInsertPageBreak.xlsx")))
}

func TestNewStreamWriter(t *testing.T) {
	// Test error exceptions
	file := NewFile()
	defer func() {
		assert.NoError(t, file.Close())
	}()
	_, err := file.NewStreamWriter("Sheet1")
	assert.NoError(t, err)
	_, err = file.NewStreamWriter("SheetN")
	assert.EqualError(t, err, "sheet SheetN does not exist")
	// Test new stream write with invalid sheet name
	_, err = file.NewStreamWriter("Sheet:1")
	assert.EqualError(t, err, ErrSheetNameInvalid.Error())
}

func TestStreamMarshalAttrs(t *testing.T) {
	var r *RowOpts
	attrs, err := r.marshalAttrs()
	assert.NoError(t, err)
	assert.Empty(t, attrs)
}

func TestStreamSetRow(t *testing.T) {
	// Test error exceptions
	file := NewFile()
	defer func() {
		assert.NoError(t, file.Close())
	}()
	streamWriter, err := file.NewStreamWriter("Sheet1")
	assert.NoError(t, err)
	assert.EqualError(t, streamWriter.SetRow("A", []interface{}{}), newCellNameToCoordinatesError("A", newInvalidCellNameError("A")).Error())
	// Test set row with non-ascending row number
	assert.NoError(t, streamWriter.SetRow("A1", []interface{}{}))
	assert.EqualError(t, streamWriter.SetRow("A1", []interface{}{}), newStreamSetRowError(1).Error())
	// Test set row with unsupported charset workbook
	file.WorkBook = nil
	file.Pkg.Store(defaultXMLPathWorkbook, MacintoshCyrillicCharset)
	assert.EqualError(t, streamWriter.SetRow("A2", []interface{}{time.Now()}), "XML syntax error on line 1: invalid UTF-8")
}

func TestStreamSetRowNilValues(t *testing.T) {
	file := NewFile()
	defer func() {
		assert.NoError(t, file.Close())
	}()
	streamWriter, err := file.NewStreamWriter("Sheet1")
	assert.NoError(t, err)
	assert.NoError(t, streamWriter.SetRow("A1", []interface{}{nil, nil, Cell{Value: "foo"}}))
	streamWriter.Flush()
	ws, err := file.workSheetReader("Sheet1")
	assert.NoError(t, err)
	assert.NotEqual(t, ws.SheetData.Row[0].C[0].XMLName.Local, "c")
}

func TestStreamSetRowWithStyle(t *testing.T) {
	file := NewFile()
	defer func() {
		assert.NoError(t, file.Close())
	}()
	zeroStyleID := 0
	grayStyleID, err := file.NewStyle(&Style{Font: &Font{Color: "#777777"}})
	assert.NoError(t, err)
	blueStyleID, err := file.NewStyle(&Style{Font: &Font{Color: "#0000FF"}})
	assert.NoError(t, err)
	
	streamWriter, err := file.NewStreamWriter("Sheet1")
	assert.NoError(t, err)
	assert.NoError(t, streamWriter.SetRow("A1", []interface{}{
		"value1",
		Cell{Value: "value2"},
		&Cell{Value: "value2"},
		Cell{StyleID: blueStyleID, Value: "value3"},
		&Cell{StyleID: blueStyleID, Value: "value3"},
	}, RowOpts{StyleID: grayStyleID}))
	err = streamWriter.Flush()
	assert.NoError(t, err)
	
	ws, err := file.workSheetReader("Sheet1")
	assert.NoError(t, err)
	assert.Equal(t, grayStyleID, ws.SheetData.Row[0].C[0].S)
	assert.Equal(t, zeroStyleID, ws.SheetData.Row[0].C[1].S)
	assert.Equal(t, zeroStyleID, ws.SheetData.Row[0].C[2].S)
	assert.Equal(t, blueStyleID, ws.SheetData.Row[0].C[3].S)
	assert.Equal(t, blueStyleID, ws.SheetData.Row[0].C[4].S)
}

func TestStreamSetCellValFunc(t *testing.T) {
	f := NewFile()
	defer func() {
		assert.NoError(t, f.Close())
	}()
	sw, err := f.NewStreamWriter("Sheet1")
	assert.NoError(t, err)
	c := &xlsxC{}
	assert.NoError(t, sw.setCellValFunc(c, 128))
	assert.NoError(t, sw.setCellValFunc(c, int8(-128)))
	assert.NoError(t, sw.setCellValFunc(c, int16(-32768)))
	assert.NoError(t, sw.setCellValFunc(c, int32(-2147483648)))
	assert.NoError(t, sw.setCellValFunc(c, int64(-9223372036854775808)))
	assert.NoError(t, sw.setCellValFunc(c, uint(128)))
	assert.NoError(t, sw.setCellValFunc(c, uint8(255)))
	assert.NoError(t, sw.setCellValFunc(c, uint16(65535)))
	assert.NoError(t, sw.setCellValFunc(c, uint32(4294967295)))
	assert.NoError(t, sw.setCellValFunc(c, uint64(18446744073709551615)))
	assert.NoError(t, sw.setCellValFunc(c, float32(100.1588)))
	assert.NoError(t, sw.setCellValFunc(c, 100.1588))
	assert.NoError(t, sw.setCellValFunc(c, " Hello"))
	assert.NoError(t, sw.setCellValFunc(c, []byte(" Hello")))
	assert.NoError(t, sw.setCellValFunc(c, time.Now().UTC()))
	assert.NoError(t, sw.setCellValFunc(c, time.Duration(1e13)))
	assert.NoError(t, sw.setCellValFunc(c, true))
	assert.NoError(t, sw.setCellValFunc(c, nil))
	assert.NoError(t, sw.setCellValFunc(c, complex64(5+10i)))
}

func TestStreamWriterOutlineLevel(t *testing.T) {
	file := NewFile()
	streamWriter, err := file.NewStreamWriter("Sheet1")
	assert.NoError(t, err)
	
	// Test set outlineLevel in row
	assert.NoError(t, streamWriter.SetRow("A1", nil, RowOpts{OutlineLevel: 1}))
	assert.NoError(t, streamWriter.SetRow("A2", nil, RowOpts{OutlineLevel: 7}))
	assert.ErrorIs(t, ErrOutlineLevel, streamWriter.SetRow("A3", nil, RowOpts{OutlineLevel: 8}))
	
	assert.NoError(t, streamWriter.Flush())
	// Save spreadsheet by the given path
	assert.NoError(t, file.SaveAs(filepath.Join("test", "TestStreamWriterSetRowOutlineLevel.xlsx")))
	
	file, err = OpenFile(filepath.Join("test", "TestStreamWriterSetRowOutlineLevel.xlsx"))
	assert.NoError(t, err)
	level, err := file.GetRowOutlineLevel("Sheet1", 1)
	assert.NoError(t, err)
	assert.Equal(t, uint8(1), level)
	level, err = file.GetRowOutlineLevel("Sheet1", 2)
	assert.NoError(t, err)
	assert.Equal(t, uint8(7), level)
	level, err = file.GetRowOutlineLevel("Sheet1", 3)
	assert.NoError(t, err)
	assert.Equal(t, uint8(0), level)
	assert.NoError(t, file.Close())
}
