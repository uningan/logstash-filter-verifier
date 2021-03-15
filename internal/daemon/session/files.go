package session

const outputPipeline = `input {
  pipeline {
    address => {{ .PipelineName }}
  }
}

filter {
  mutate {
    add_tag => [ "__lfv_out_{{ .PipelineOrigName }}_passed" ]
  }
}

output {
  pipeline {
    send_to => __lfv_output
  }
}
`

const inputGenerator = `
input {
  generator {
    lines => [
      {{ .InputLines }}
    ]
    {{ .InputCodec }}
    count => 1
    threads => 1
  }
}

filter {
  ruby {
    id => '__lfv_ruby_count'
    init => '@count = 0'
    code => 'event.set("__lfv_id", @count.to_s)
             @count += 1'
    tag_on_exception => '__lfv_ruby_count_exception'
  }

  mutate {
    add_tag => [ "__lfv_in_passed" ]
    # Remove fields "host", "sequence" and optionally "message", which are
    # automatically created by the generator input.
    remove_field => [ {{ .RemoveGeneratorFields }} ]
  }

  translate {
    dictionary_path => "{{ .FieldsFilename }}"
    field => "[__lfv_id]"
    destination => "[@metadata][__lfv_fields]"
    exact => true
    override => true
    # TODO: Add default value (e.g. "__lfv_fields_not_found"), if not found in dictionary
  }

  ruby {
    id => '__lfv_ruby_fields'
    # TODO: If default value ("__lfv_fields_not_found"), then skip this ruby
    # code and add an tag instead
    code => 'fields = event.get("[@metadata][__lfv_fields]")
             fields.each { |key, value| event.set(key, value) }'
    tag_on_exception => '__lfv_ruby_fields_exception'
  }
}

output {
  pipeline {
    send_to => [ "{{ .InputPluginName }}" ]
  }
}
`
