UPDATE problems
SET title              = COALESCE($2, title),
    time_limit         = COALESCE($3, time_limit),
    memory_limit       = COALESCE($4, memory_limit),
    visibility         = COALESCE($5, visibility),
    legend             = COALESCE($6, legend),
    input_format       = COALESCE($7, input_format),
    output_format      = COALESCE($8, output_format),
    notes              = COALESCE($9, notes),
    scoring            = COALESCE($10, scoring),
    legend_html        = COALESCE($11, legend_html),
    input_format_html  = COALESCE($12, input_format_html),
    output_format_html = COALESCE($13, output_format_html),
    notes_html         = COALESCE($14, notes_html),
    scoring_html       = COALESCE($15, scoring_html)
WHERE id = $1
