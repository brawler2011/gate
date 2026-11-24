-- Statements are read back by pandoc's LaTeX reader and printed by plain LaTeX: neither knows \tightlist or longtable.
local function latex(blocks)
  return (pandoc.write(pandoc.Pandoc(blocks), "latex"):gsub("%s+$", ""))
end

local function loosen(list)
  for _, item in ipairs(list.content) do
    for i, block in ipairs(item) do
      if block.t == "Plain" then
        item[i] = pandoc.Para(block.content)
      end
    end
  end
  return list
end

BulletList = loosen
OrderedList = loosen

local columns = {AlignLeft = "l", AlignRight = "r", AlignCenter = "c", AlignDefault = "l"}

function Table(tbl)
  local t = pandoc.utils.to_simple_table(tbl)
  local spec, rows = {}, {}
  for _, a in ipairs(t.aligns) do
    spec[#spec + 1] = columns[a] or "l"
  end
  local function row(cells)
    local out = {}
    for _, cell in ipairs(cells) do
      out[#out + 1] = latex(cell)
    end
    rows[#rows + 1] = table.concat(out, " & ") .. " \\\\ \\hline"
  end
  local header = false
  for _, cell in ipairs(t.headers) do
    header = header or #cell > 0
  end
  if header then
    row(t.headers)
  end
  for _, r in ipairs(t.rows) do
    row(r)
  end
  return pandoc.RawBlock("latex", "\\begin{tabular}{|" .. table.concat(spec, "|") .. "|}\\hline\n" .. table.concat(rows, "\n") .. "\n\\end{tabular}")
end
